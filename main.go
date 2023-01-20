package main

import (
	"github.com/rs/zerolog/log"
	"os"
)

func main() {
	command, options, err := ParseOptions()
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse options")
	}
	if command == NoCommand {
		os.Exit(1)
	} else if command == SecretsCommand {
		ScanSecrets(options)
	} else if command == CTCommand {
		config := CTConfig{URL: "https://oak.ct.letsencrypt.org/2023/", GetEntriesBatchSize: 256, GetEntriesRetries: 5}
		GetCTInstances(&config)
	} else if command == FilterInstanceCommand {
		FilterForGitLab()
	}
	os.Exit(1)

}
