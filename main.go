package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	db, err := NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not open database")
	}
	db.Close()

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	command := GetSubcommand()
	if command == CTCommand {
		config := CTConfig{URL: "https://oak.ct.letsencrypt.org/2023/", GetEntriesBatchSize: 256, GetEntriesRetries: 5}
		GetCTInstances(&config)
	} else if command == FilterInstanceCommand {
		FilterForGitLab()
	} else if command == SecretsCommand {
		RunSecrets()
	} else {
		log.Fatal().Msg("unknown command. options are: ct, filter, secrets")
	}
}
