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
	Instance    *Instance
	GitlabFound bool
	AllowSignup bool
	Email       string
	Password    string
}

func FilterInputWorker(instanceChan chan<- Instance) {
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

func FilterProcessWorker(instanceChan <-chan Instance, resultChan chan<- FilterResult) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	for {
		instance, ok := <-instanceChan
		if !ok {
			return
		}

		found := false
		resp, err := client.Get(fmt.Sprintf("https://%s%s", instance.Name, GitlabMagicURL))
		if err != nil {
			log.Info().Str("instance", instance.Name).Err(err).Msg("error requesting instance")
		} else if resp.StatusCode != 200 {
			log.Info().Str("instance", instance.Name).Int("status", resp.StatusCode).Msg("no instance found")
		} else {
			var body []byte
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				log.Info().Str("instance", instance.Name).Err(err).Msg("error while reading body")
			}
			bodyStr := string(body)
			if strings.Contains(bodyStr, GitlabMagicString) {
				log.Info().Str("instance", instance.Name).Msg("gitlab instance found")
				found = true
				resultChan <- FilterResult{Instance: &instance, GitlabFound: true, AllowSignup: strings.Contains(bodyStr, GitlabRegisterMagicString)}
			}
		}

		if !found {
			resultChan <- FilterResult{Instance: &instance, GitlabFound: false}
		}
	}
}

func FilterOutputWorker(resultsChan <-chan FilterResult) {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	for {
		result, ok := <-resultsChan
		if !ok {
			return
		}
		if result.GitlabFound {
			err = db.AddGitLab(GitLab{
				InstanceID:  result.Instance.ID,
				AllowSignup: result.AllowSignup,
				Email:       "",
				Password:    "",
				Processed:   false,
			})
			if err != nil {
				log.Fatal().Str("instance", result.Instance.Name).Err(err).Msg("could not insert gitlab into db")
			}
		}
		err = db.SetInstanceProcessed(result.Instance.ID)
		if err != nil {
			log.Fatal().Str("instance", result.Instance.Name).Err(err).Msg("could not set instance processed")
		}
	}

}

const FilterWorkers = 20

func FilterForGitLab() {
	Fan[Instance, FilterResult]{
		InputWorker:   FilterInputWorker,
		ProcessWorker: FilterProcessWorker,
		OutputWorker:  FilterOutputWorker,
		Workers:       FilterWorkers,
		InputBuffer:   100,
		OutputBuffer:  100,
	}.Run()
}
