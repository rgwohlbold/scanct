package main

import (
	"context"
	"database/sql"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"math"
	"net/http"
	"sync"
	"time"
)

type CTConfig struct {
	URL                 string
	GetEntriesRetries   int
	GetEntriesBatchSize int
}

type Certificate struct {
	Subjects []string
	Index    int64
}

type Database struct {
	db *sql.DB
}

func NewDatabase() (Database, error) {
	db, err := sql.Open("sqlite3", "./instances.db")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not open database")
	}
	_, err = db.Exec("create table if not exists instances (id integer not null primary key, ind integer, name text);")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not create table")
	}
	_, err = db.Exec("create unique index if not exists ind_idx on instances (ind);")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not create index")
	}
	return Database{db}, nil
}

func (d *Database) Close() {
	err := d.db.Close()
	if err != nil {
		log.Error().Err(err).Msg("error closing database")
	}
}

func (d *Database) IndexRange() (int64, int64, error) {
	res, err := d.db.Query("select count(*) from instances")
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not count rows")
	}
	res.Next()
	var rows int64
	err = res.Scan(&rows)
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not scan row")
	}
	if rows == 0 {
		return math.MaxInt64 / 2, math.MaxInt64 / 2, nil
	}

	res, err = d.db.Query("select max(ind), min(ind) from instances")
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not get index range")
	}
	res.Next()
	var max, min int64
	err = res.Scan(&max, &min)
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not scan row")
	}
	err = res.Close()
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not close results")
	}
	return min, max, nil
}

func (d *Database) StoreCertificates(certs []Certificate) {
	var tx *sql.Tx
	tx, err := d.db.Begin()
	if err != nil {
		log.Fatal().Err(err).Msg("could not begin transaction")
	}
	var stmt *sql.Stmt
	stmt, err = tx.Prepare("insert into instances(ind, name) values(?, ?)")
	if err != nil {
		log.Fatal().Err(err).Msg("could not prepare statement")
	}
	for _, cert := range certs {
		for _, subject := range cert.Subjects {
			_, err = stmt.Exec(cert.Index, subject)
			if err != nil {
				log.Fatal().Err(err).Msg("could not execute statement")
			}
		}

	}
	err = tx.Commit()
	if err != nil {
		log.Fatal().Err(err).Msg("could not commit transaction")
	}
}

func ConnectLog(config *CTConfig) (*client.LogClient, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   10,
			DisableKeepAlives:     false,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	return client.New(config.URL, httpClient, jsonclient.Options{})
}

func GitlabInstanceWorker(config *CTConfig, startChan <-chan int64, certChan chan<- []Certificate) {
	ctx := context.Background()
	c, err := ConnectLog(config)
	if err != nil {
		log.Fatal().Err(err)
	}
	for {
		start, ok := <-startChan
		if !ok {
			return
		}
		end := start + int64(config.GetEntriesBatchSize)
		var entries []ct.LogEntry

		for i := 0; i < config.GetEntriesRetries; i++ {
			entries, err = c.GetEntries(ctx, start, end)
			if err == nil {
				break
			}
			time.Sleep(1)

		}
		if err != nil {
			log.Error().Err(err).Msg("error in get-entries")
		}
		certs := make([]Certificate, config.GetEntriesBatchSize)
		for i, entry := range entries {
			certs[i] = Certificate{Index: entry.Index, Subjects: make([]string, 0)}
			cert, err := entry.Leaf.X509Certificate()
			if err != nil {
				log.Fatal().Err(err)
			}
			if cert != nil {
				certs[i].Subjects = append(certs[i].Subjects, cert.Subject.CommonName)
			} else {
				precert, err := entry.Leaf.Precertificate()
				if err != nil {
					log.Fatal().Err(err)
				}
				certs[i].Subjects = append(certs[i].Subjects, precert.Subject.CommonName)
			}
		}
		certChan <- certs
	}
}

func DBWorker(config *CTConfig, certChan <-chan []Certificate, doneChan <-chan bool) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	defer db.Close()

	k := 0
	for {
		select {
		case certs, ok := <-certChan:
			if !ok {
				return
			}
			if len(certs) != config.GetEntriesBatchSize {
				log.Fatal().Msg("not exactly GetEntriesBatchSize certificates arrived")
			}
			k += len(certs)
			db.StoreCertificates(certs)
			log.Info().Int("certs", k).Msg("processed certs")
		case <-doneChan:
			return
		}
	}
}

func StartIndexWorker(config *CTConfig, startChan chan<- int64) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not open database")
	}
	minIndex, maxIndex, err := db.IndexRange()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get index range")
	}
	ctx := context.Background()
	c, err := ConnectLog(config)
	if err != nil {
		log.Fatal().Err(err)
	}
	sth, err := c.GetSTH(ctx)
	if err != nil {
		log.Fatal().Err(err)
	}

	maxLogIndex := int64(sth.TreeSize - 1)

	// catch up
	index := maxIndex + 1
	log.Info().Int64("certs", maxLogIndex-index+1).Msg("catching up to sth")
	for index <= maxLogIndex {
		startChan <- index
		index += 256
	}
	log.Info().Msg("done catching up")
	// go back
	index = maxLogIndex
	if minIndex < maxLogIndex {
		index = minIndex
	}
	for {
		startChan <- index
		index -= 256
	}
}

const Workers = 30

func StartInstanceWorkers(config *CTConfig) {
	var wg sync.WaitGroup
	var dbWg sync.WaitGroup

	wg.Add(Workers)

	startChan := make(chan int64, Workers)
	subjectsChan := make(chan []Certificate, Workers)
	doneChan := make(chan bool)
	go StartIndexWorker(config, startChan)
	for i := 0; i < Workers; i++ {
		go func() {
			GitlabInstanceWorker(config, startChan, subjectsChan)
		}()
	}

	dbWg.Add(1)
	go func() {
		DBWorker(config, subjectsChan, doneChan)
		dbWg.Done()
	}()

	wg.Wait()
	doneChan <- true
	dbWg.Done()

	close(subjectsChan)
}
