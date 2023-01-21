package main

import (
	"os"
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
