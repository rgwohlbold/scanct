package main

import (
	"context"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"net/http"
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

func CTProcessWorker(config *CTConfig, startChan <-chan int64, certChan chan<- []Certificate) {
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

func CTOutputWorker(config *CTConfig, certChan <-chan []Certificate) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	defer db.Close()

	k := 0
	for {
		certs, ok := <-certChan
		if !ok {
			return
		}
		if len(certs) != config.GetEntriesBatchSize {
			log.Fatal().Msg("not exactly GetEntriesBatchSize certificates arrived")
		}
		k += len(certs)
		db.StoreCertificates(certs)
		log.Info().Int("certs", k).Msg("processed certs")
	}
}

func CTInputWorker(config *CTConfig, startChan chan<- int64) {
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

const CTWorkers = 30

func GetCTInstances(config *CTConfig) {
	Fan[int64, []Certificate]{
		InputWorker: func(inputChan chan<- int64) {
			CTInputWorker(config, inputChan)
		},
		ProcessWorker: func(inputChan <-chan int64, outputChan chan<- []Certificate) {
			CTProcessWorker(config, inputChan, outputChan)
		},
		OutputWorker: func(outputChan <-chan []Certificate) {
			CTOutputWorker(config, outputChan)
		},
		Workers:      CTWorkers,
		InputBuffer:  100,
		OutputBuffer: 100,
	}.Run()
}
