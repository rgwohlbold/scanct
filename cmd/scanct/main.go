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

type Subcommand int

const (
	CTCommand             Subcommand = iota
	FilterInstanceCommand            = iota
	SecretsCommand                   = iota
	RepositoryCommand                = iota
	NoCommand                        = iota
)

func GetSubcommand() Subcommand {
	if len(os.Args) < 2 {
		return NoCommand
	}
	commandMap := map[string]Subcommand{
		"ct":         CTCommand,
		"filter":     FilterInstanceCommand,
		"secrets":    SecretsCommand,
		"repository": RepositoryCommand,
	}
	command, ok := commandMap[os.Args[1]]
	if !ok {
		return NoCommand
	}
	return command
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

	command := GetSubcommand()
	if command == CTCommand {
		config := scanct.CTConfig{URL: "https://oak.ct.letsencrypt.org/2023/", GetEntriesBatchSize: 256, GetEntriesRetries: 5, NumCerts: math.MaxInt64}
		if len(os.Args) >= 3 {
			config.NumCerts, err = strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil {
				log.Fatal().Err(err).Msg("could not parse number of certs")
			}
		}
		scanct.RunCTCommand(&config)
	} else if command == FilterInstanceCommand {
		if len(os.Args) < 3 {
			log.Fatal().Msg("missing filter name, either gitlab or jenkins")
		}
		filterName := os.Args[2]
		if filterName == "gitlab" {
			gitlab.RunFilterGitlabCommand()
		} else if filterName == "jenkins" {
			jenkins.RunFilterJenkinsCommand()
		} else {
			log.Fatal().Msg("unknown filter name, either gitlab or jenkins")
		}
	} else if command == RepositoryCommand {
		if len(os.Args) < 3 {
			log.Fatal().Msg("missing repository name, either gitlab or jenkins")
		}
		repositoryName := os.Args[2]
		if repositoryName == "gitlab" {
			gitlab.RunRepositoryCommand()
		} else if repositoryName == "jenkins" {
			jenkins.RunJenkinsProcessor()
		} else {
			log.Fatal().Msg("unknown repository name, either gitlab or jenkins")
		}
	} else if command == SecretsCommand {
		if len(os.Args) < 3 {
			log.Fatal().Msg("missing secrets name, either gitlab or jenkins")
		}
		secretsName := os.Args[2]
		if secretsName == "gitlab" {
			gitlab.RunSecretsCommand()
		} else if secretsName == "jenkins" {
			jenkins.RunJenkinsSecretsFinder()
		} else {
			log.Fatal().Msg("unknown secrets name, either gitlab or jenkins")
		}
	} else {
		log.Fatal().Msg("unknown command. options are: ct, filter, repository, secrets")
	}
}
