package main

import (
	"os"
)

type Subcommand int

const (
	CTCommand             Subcommand = iota
	FilterInstanceCommand            = iota
	SecretsCommand                   = iota
	NoCommand                        = iota
)

func GetSubcommand() Subcommand {
	if len(os.Args) < 2 {
		return NoCommand
	}
	if os.Args[1] == "secrets" {
		return SecretsCommand
	} else if os.Args[1] == "ct" {
		return CTCommand
	} else if os.Args[1] == "filter" {
		return FilterInstanceCommand
	}
	return NoCommand
}
