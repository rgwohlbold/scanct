package main

import (
	"context"
	"fmt"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/rs/zerolog/log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type CertData struct {
	LeafInput string `json:"leaf_input"`
	ExtraData string `json:"extra_data"`
}

type CertLog struct {
	Entries []CertData
}

func ConnectLog(session *Session) (*client.LogClient, error) {
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
	return client.New(session.Config.CertificateLogURI, httpClient, jsonclient.Options{})
}

func GitlabInstanceWorker(session *Session, startChan <-chan int64, subjectChan chan<- []string) {
	ctx := context.Background()
	c, err := ConnectLog(session)
	if err != nil {
		log.Fatal().Err(err)
	}
	for {
		start, ok := <-startChan
		if !ok {
			return
		}
		end := start + 256
		entries, err := c.GetEntries(ctx, start, end)
		if err != nil {
			log.Fatal().Err(err)
		}
		subjects := make([]string, 256)
		for i, entry := range entries {
			cert, err := entry.Leaf.X509Certificate()
			if err != nil {
				log.Fatal().Err(err)
			}
			if cert != nil {
				subjects[i] = cert.Subject.CommonName
				fmt.Println(entry.Index, "cert", cert.Subject)
			} else {
				precert, err := entry.Leaf.Precertificate()
				if err != nil {
					log.Fatal().Err(err)
				}
				fmt.Println(entry.Index, "precert", precert.Subject, precert.Subject.Names)
				subjects[i] = precert.Subject.CommonName
			}
		}
		subjectChan <- subjects
	}
}

func ProcessInstancesWorker(session *Session, subjectsChan <-chan []string) {
	k := 0
	for {
		subjects, ok := <-subjectsChan
		if !ok {
			return
		}
		if len(subjects) != 256 {
			fmt.Println("not good")
		}
		k += len(subjects)
		for _, subject := range subjects {
			if strings.Contains(subject, "git") {
				fmt.Println(subject)
			}
		}
		fmt.Println(k)
	}
}

func StartIndexWorker(session *Session, startChan chan<- int64) {
	ctx := context.Background()
	c, err := ConnectLog(session)
	if err != nil {
		log.Fatal().Err(err)
	}
	sth, err := c.GetSTH(ctx)
	if err != nil {
		log.Fatal().Err(err)
	}
	startIndex := int64(sth.TreeSize - 1)
	startIndex = startIndex - startIndex%256

	for {
		startChan <- startIndex
		startIndex -= 256
	}
}

const Workers = 50

func StartInstanceWorkers(session *Session) {
	var wg sync.WaitGroup

	wg.Add(Workers)

	startChan := make(chan int64, Workers)
	subjectsChan := make(chan []string, Workers)
	go StartIndexWorker(session, startChan)
	for i := 0; i < Workers; i++ {
		go func() {
			GitlabInstanceWorker(session, startChan, subjectsChan)
		}()
	}
	go ProcessInstancesWorker(session, subjectsChan)
	wg.Wait()
	close(subjectsChan)
}
