package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math"
	"os"
	"runtime"
	"strconv"
)

func main() {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not open database")
	}
	db.Close()

	runtime.GOMAXPROCS(runtime.NumCPU())

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	command := GetSubcommand()
	if command == CTCommand {
		config := CTConfig{URL: "https://oak.ct.letsencrypt.org/2023/", GetEntriesBatchSize: 256, GetEntriesRetries: 5, NumCerts: math.MaxInt64}
		if len(os.Args) >= 3 {
			config.NumCerts, err = strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil {
				log.Fatal().Err(err).Msg("could not parse number of certs")
			}
		}
		RunCTCommand(&config)
	} else if command == FilterInstanceCommand {
		if len(os.Args) < 3 {
			log.Fatal().Msg("missing filter name, either gitlab or jenkins")
		}
		filterName := os.Args[2]
		if filterName == "gitlab" {
			RunFilterGitlabCommand()
		} else if filterName == "jenkins" {
			RunFilterJenkinsCommand()
		} else {
			log.Fatal().Msg("unknown filter name, either gitlab or jenkins")
		}
	} else if command == RepositoryCommand {
		if len(os.Args) < 3 {
			log.Fatal().Msg("missing repository name, either gitlab or jenkins")
		}
		repositoryName := os.Args[2]
		if repositoryName == "gitlab" {
			RunRepositoryCommand()
		} else if repositoryName == "jenkins" {
			RunJenkinsProcessor()
		} else {
			log.Fatal().Msg("unknown repository name, either gitlab or jenkins")
		}
	} else if command == SecretsCommand {
		RunSecretsCommand()
	} else {
		log.Fatal().Msg("unknown command. options are: ct, filter, repository, secrets")
	}
}
