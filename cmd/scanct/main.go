package main

import (
	"github.com/rgwohlbold/scanct"
	"github.com/rgwohlbold/scanct/aws"
	"github.com/rgwohlbold/scanct/gitlab"
	"github.com/rgwohlbold/scanct/jenkins"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math"
	"os"
	"runtime"
	"strconv"
	"time"
)

func FullProcess() {
	gitlab.FilterInstances()
	jenkins.FilterInstances()
	gitlab.ImportRepositories()
	jenkins.ImportJobs()
	gitlab.ScanSecrets()
	jenkins.ScanSecrets()
	aws.RunGitlabKeysStep()
	aws.RunJenkinsKeysStep()
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	runtime.GOMAXPROCS(runtime.NumCPU())

	db, err := scanct.NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not open database")
	}
	db.Close()

	if len(os.Args) < 2 {
		log.Fatal().Msg("no subcommand given. choose either 'ct', 'jenkins', 'gitlab'.")
	}
	if os.Args[1] == "ct" {
		config := scanct.CTConfig{URL: "https://oak.ct.letsencrypt.org/2023/", GetEntriesBatchSize: 256, GetEntriesRetries: 5, NumCerts: math.MaxInt64}
		if len(os.Args) >= 3 {
			config.NumCerts, err = strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil {
				log.Fatal().Err(err).Msg("could not parse number of certs")
			}
		}
		scanct.ImportCertificates(&config)
	} else if os.Args[1] == "jenkins" {
		if len(os.Args) < 3 {
			log.Fatal().Msg("no action given. choose either 'filter', 'jobs' or 'secrets'.")
		}
		if os.Args[2] == "filter" {
			jenkins.FilterInstances()
		} else if os.Args[2] == "jobs" {
			jenkins.ImportJobs()
		} else if os.Args[2] == "secrets" {
			jenkins.ScanSecrets()
		} else if os.Args[2] == "aws" {
			aws.RunJenkinsKeysStep()
		} else {
			log.Fatal().Msg("unknown action. choose either 'filter', 'jobs' or 'secrets'.")
		}
	} else if os.Args[1] == "gitlab" {
		if len(os.Args) < 3 {
			log.Fatal().Msg("no action given. choose either 'filter', 'repositories' or 'secrets'.")
		}
		if os.Args[2] == "filter" {
			gitlab.FilterInstances()
		} else if os.Args[2] == "repositories" {
			gitlab.ImportRepositories()
		} else if os.Args[2] == "secrets" {
			gitlab.ScanSecrets()
		} else if os.Args[2] == "aws" {
			aws.RunGitlabKeysStep()
		} else {
			log.Fatal().Msg("unknown action. choose either 'filter', 'repositories' or 'secrets'.")
		}
	} else if os.Args[1] == "full" {
		config := scanct.CTConfig{URL: "https://oak.ct.letsencrypt.org/2023/", GetEntriesBatchSize: 256, GetEntriesRetries: 5, NumCerts: math.MaxInt64}
		if len(os.Args) >= 3 {
			config.NumCerts, err = strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil {
				log.Fatal().Err(err).Msg("could not parse number of certs")
			}
			scanct.ImportCertificates(&config)
			FullProcess()
		} else {
			FullProcess()
			config.NumCerts = 10000000
			for {
				// Make each iteration take at least 5 minutes to avoid spamming the CT logs
				minTimeChannel := time.After(5 * time.Minute)
				log.Info().Msg("starting daemon iteration")

				scanct.ImportCertificates(&config)
				FullProcess()

				log.Info().Msg("waiting for next daemon iteration")
				<-minTimeChannel
			}
		}
	} else {
		log.Fatal().Msg("unknown subcommand. choose either 'ct', 'jenkins', 'gitlab'.")
	}
}
