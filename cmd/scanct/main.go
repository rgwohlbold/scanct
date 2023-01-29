package main

import (
	"github.com/rgwohlbold/scanct"
	"github.com/rgwohlbold/scanct/gitlab"
	"github.com/rgwohlbold/scanct/jenkins"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math"
	"os"
	"runtime"
	"strconv"
)

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
			log.Fatal().Msg("no action given. choose either 'import', 'jobs' or 'scan'.")
		}
		if os.Args[2] == "import" {
			jenkins.RunFilterJenkinsCommand()
		} else if os.Args[2] == "jobs" {
			jenkins.RunJenkinsProcessor()
		} else if os.Args[2] == "scan" {
			jenkins.RunJenkinsSecretsFinder()
		} else {
			log.Fatal().Msg("unknown action. choose either 'import', 'jobs' or 'scan'.")
		}
	} else if os.Args[1] == "gitlab" {
		if len(os.Args) < 3 {
			log.Fatal().Msg("no action given. choose either 'import', 'jobs' or 'scan'.")
		}
		if os.Args[2] == "import" {
			gitlab.RunFilterGitlabCommand()
		} else if os.Args[2] == "jobs" {
			gitlab.RunRepositoryCommand()
		} else if os.Args[2] == "scan" {
			gitlab.RunSecretsCommand()
		} else {
			log.Fatal().Msg("unknown action. choose either 'import', 'jobs' or 'scan'.")
		}
	} else {
		log.Fatal().Msg("unknown command. options are: ct, filter, repository, secrets")
	}
}
