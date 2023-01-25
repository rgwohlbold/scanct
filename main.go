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
		RunFilterGitlabCommand()
	} else if command == RepositoryCommand {
		RunRepositoryCommand()
	} else if command == SecretsCommand {
		RunSecretsCommand()
	} else {
		log.Fatal().Msg("unknown command. options are: ct, filter, repository, secrets")
	}
}
