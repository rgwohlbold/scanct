package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strings"
	"time"
)

const GitlabMagicURL = "/users/sign_in"
const GitlabMagicString = "<meta content=\"GitLab\" property=\"og:site_name\">"
const GitlabRegisterMagicString = "<a data-qa-selector=\"register_link\" href=\"/users/sign_up\">Register now</a>"

type FilterResult struct {
	InstanceID  int
	GitlabFound bool
	AllowSignup bool
	Email       string
	Password    string
}

func FilterInputWorker(instanceChan chan<- UnprocessedInstance) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	potentialInstances, err := db.GetUnprocessedPotentialGitLabs()
	db.Close()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get unprocessed instances")
	}
	log.Info().Int("count", len(potentialInstances)).Msg("fetched unprocessed instances")
	for _, instance := range potentialInstances {
		instanceChan <- instance
	}
	close(instanceChan)
}

func FilterProcessWorker(instanceChan <-chan UnprocessedInstance, resultChan chan<- FilterResult) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	for {
		instance, ok := <-instanceChan
		if !ok {
			return
		}

		found := false
		resp, err := client.Get(fmt.Sprintf("https://%s%s", instance.name, GitlabMagicURL))
		if err != nil {
			log.Info().Str("instance", instance.name).Err(err).Msg("error requesting instance")
		} else if resp.StatusCode != 200 {
			log.Info().Str("instance", instance.name).Int("status", resp.StatusCode).Msg("no instance found")
		} else {
			var body []byte
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				log.Info().Str("instance", instance.name).Err(err).Msg("error while reading body")
			}
			bodyStr := string(body)
			if strings.Contains(bodyStr, GitlabMagicString) {
				log.Info().Str("instance", instance.name).Msg("gitlab instance found")
				found = true
				resultChan <- FilterResult{InstanceID: instance.id, GitlabFound: true, AllowSignup: strings.Contains(bodyStr, GitlabRegisterMagicString)}
			}
		}

		if !found {
			resultChan <- FilterResult{InstanceID: instance.id, GitlabFound: false}
		}
	}
}

func FilterOutputWorker(resultsChan <-chan FilterResult) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	defer db.Close()
	for {
		instance, ok := <-resultsChan
		if !ok {
			return
		}
		if instance.GitlabFound {
			err = db.AddGitLab(instance.InstanceID, instance.AllowSignup, "", "")
			if err != nil {
				log.Fatal().Int("instance", instance.InstanceID).Err(err).Msg("could not insert gitlab into db")
			}
		} else {
			err = db.SetProcessed(instance.InstanceID)
			if err != nil {
				log.Fatal().Int("instance", instance.InstanceID).Err(err).Msg("could not set to processed")
			}
		}
	}

}

const FilterWorkers = 20

func FilterForGitLab() {
	Fan[UnprocessedInstance, FilterResult]{
		InputWorker:   FilterInputWorker,
		ProcessWorker: FilterProcessWorker,
		OutputWorker:  FilterOutputWorker,
		Workers:       FilterWorkers,
		InputBuffer:   100,
		OutputBuffer:  100,
	}.Run()
}
