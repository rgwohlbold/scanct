package core

import (
	"flag"
	"os"
	"path/filepath"
)

type Options struct {
	Threads                *int
	Debug                  *bool
	MaximumRepositorySize  *uint
	MaximumFileSize        *uint
	CloneRepositoryTimeout *uint
	EntropyThreshold       *float64
	PathChecks             *bool
	TempDirectory          *string
	SearchQuery            *string
}

func ParseOptions() (*Options, error) {
	options := &Options{
		Threads:                flag.Int("threads", 0, "Number of concurrent threads (default number of logical CPUs)"),
		Debug:                  flag.Bool("debug", false, "Print debugging information"),
		MaximumRepositorySize:  flag.Uint("maximum-repository-size", 5120, "Maximum repository size to process in KB"),
		MaximumFileSize:        flag.Uint("maximum-file-size", 256, "Maximum file size to process in KB"),
		CloneRepositoryTimeout: flag.Uint("clone-repository-timeout", 10, "Maximum time it should take to clone a repository in seconds. Increase this if you have a slower connection"),
		EntropyThreshold:       flag.Float64("entropy-threshold", 5.0, "Set to 0 to disable entropy checks"),
		PathChecks:             flag.Bool("path-checks", true, "Set to false to disable checking of filepaths, i.e. just match regex patterns of file contents"),
		TempDirectory:          flag.String("temp-directory", filepath.Join(os.TempDir(), "shhgit"), "Directory to process and store repositories/matches"),
		SearchQuery:            flag.String("search-query", "", "Specify a search string to ignore signatures and filter on files containing this string (regex compatible)"),
	}

	flag.Parse()

	return options, nil
}
