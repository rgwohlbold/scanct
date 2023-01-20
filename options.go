package main

import (
	"flag"
	"os"
)

type Subcommand int

const (
	CTCommand             Subcommand = iota
	FilterInstanceCommand            = iota
	SecretsCommand                   = iota
	NoCommand                        = iota
)

func ParseOptions() (Subcommand, *ScanRepositoriesConfig, error) {
	if len(os.Args) < 2 {
		return NoCommand, nil, nil
	}
	if os.Args[1] == "secrets" {
		secretsCmd := flag.NewFlagSet("secrets", flag.ExitOnError)

		instanceFlag := secretsCmd.String("instance", "", "GitLab instance to scan")
		tokenFlag := secretsCmd.String("token", "", "API token")
		err := secretsCmd.Parse(os.Args[2:])
		if *instanceFlag == "" {
			secretsCmd.Usage()
			os.Exit(1)
		}
		if err != nil {
			return NoCommand, nil, err
		}
		return SecretsCommand, &ScanRepositoriesConfig{
			Instance: &GitlabInstance{
				GitlabID: -1,
				Domain:   *instanceFlag,
			},
			TempDirectory:  "/tmp",
			GitLabApiToken: *tokenFlag,
		}, err
	} else if os.Args[1] == "ct" {
		return CTCommand, nil, nil
	} else if os.Args[1] == "filter" {
		return FilterInstanceCommand, nil, nil
	}
	return NoCommand, nil, nil
}
