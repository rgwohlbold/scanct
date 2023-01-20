package main

import (
	"flag"
	"os"
	"path/filepath"
)

type Subcommand int

const (
	SecretsCommand Subcommand = iota
	CTCommand                 = iota
	NoCommand                 = iota
)

type SecretsConfig struct {
	Threads                *int
	Debug                  *bool
	MaximumRepositorySize  *uint
	MaximumFileSize        *uint
	CloneRepositoryTimeout *uint
	EntropyThreshold       *float64
	PathChecks             *bool
	TempDirectory          *string
}

func ParseOptions() (Subcommand, *SecretsConfig, error) {
	secretsCmd := flag.NewFlagSet("secrets", flag.ExitOnError)
	options := &SecretsConfig{
		Threads:                secretsCmd.Int("threads", 0, "Number of concurrent threads (default number of logical CPUs)"),
		Debug:                  secretsCmd.Bool("debug", false, "Print debugging information"),
		MaximumRepositorySize:  secretsCmd.Uint("maximum-repository-size", 5120, "Maximum repository size to process in KB"),
		MaximumFileSize:        secretsCmd.Uint("maximum-file-size", 256, "Maximum file size to process in KB"),
		CloneRepositoryTimeout: secretsCmd.Uint("clone-repository-timeout", 10, "Maximum time it should take to clone a repository in seconds. Increase this if you have a slower connection"),
		EntropyThreshold:       secretsCmd.Float64("entropy-threshold", 5.0, "Set to 0 to disable entropy checks"),
		PathChecks:             secretsCmd.Bool("path-checks", true, "Set to false to disable checking of filepaths, i.e. just match regex patterns of file contents"),
		TempDirectory:          secretsCmd.String("temp-directory", filepath.Join(os.TempDir(), "shhgit"), "Directory to process and store repositories/matches"),
	}

	if len(os.Args) < 2 {
		return NoCommand, nil, nil
	}
	if os.Args[1] == "secrets" {
		err := secretsCmd.Parse(os.Args[2:])
		return SecretsCommand, options, err
	} else if os.Args[1] == "ct" {
		return CTCommand, nil, nil
	}
	return NoCommand, nil, nil
}
