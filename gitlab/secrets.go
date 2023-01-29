package gitlab

import (
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rgwohlbold/scanct"
	"github.com/rs/zerolog/log"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxItemCountPerPage = 100 // 100 is the maximum defined by the GitLab API

func CloneRepository(r *scanct.Repository, dir string) (*git.Repository, error) {
	localCtx, cancel := context.WithTimeout(context.Background(), CloneRepositoryTimeout)
	defer cancel()

	opts := &git.CloneOptions{
		Depth:             1,
		RecurseSubmodules: git.NoRecurseSubmodules,
		URL:               r.CloneURL(),
		SingleBranch:      false,
		Tags:              git.NoTags,
	}
	if r.GitLab.APIToken != "" {
		opts.Auth = &http.BasicAuth{Username: "git", Password: r.GitLab.APIToken}
	}

	repository, err := git.PlainCloneContext(localCtx, dir, false, opts)

	if err != nil {
		return nil, errors.Wrap(err, "failed to clone repository")
	}

	return repository, nil
}

func RepositoryInputWorker(inputChan chan<- scanct.Repository) {
	defer close(inputChan)

	db, err := scanct.NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	defer db.Close()

	repositories, err := db.GetUnprocessedRepositories()
	rand.Shuffle(len(repositories), func(i, j int) {
		repositories[i], repositories[j] = repositories[j], repositories[i]
	})
	for _, repository := range repositories {
		inputChan <- repository
	}
}

func FindingsForRepository(dir string) ([]report.Finding, error) {
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create detector")
	}
	findings, err := detector.DetectGit(dir, "", detect.DetectType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect findings")
	}
	_findings := make([]report.Finding, 0, len(findings))
	for _, finding := range findings {
		found := false
		for _, f := range _findings {
			if f.Secret == finding.Secret {
				found = true
				break
			}
		}
		if !found {
			_findings = append(_findings, finding)
		}
	}
	return _findings, nil
}

func RepositoryProcessWorker(inputChan <-chan scanct.Repository, outputChan chan<- scanct.Finding) {
	db, err := scanct.NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	defer db.Close()

	for {
		repository, ok := <-inputChan
		if !ok {
			return
		}
		url := fmt.Sprintf("%s/%s", repository.GitLab.BaseURL, repository.Name)
		log.Debug().Str("repository", url).Msg("processing repository")

		dir := filepath.Join("/tmp", scanct.Hash(url))
		var res os.FileInfo
		res, err = os.Stat(dir)
		if err != nil && !os.IsNotExist(err) {
			log.Fatal().Err(err).Msg("could not stat clone dir")
		} else if err == nil && res.IsDir() {
			err = os.RemoveAll(dir)
			if err != nil {
				log.Fatal().Err(err).Msg("could not remove existing clone dir")
			}
		}
		log.Debug().Str("repository", url).Str("temp_directory", dir).Msg("cloning repository")
		_, err = CloneRepository(&repository, dir)

		if err != nil {
			log.Warn().Str("repository", url).Err(err).Msg("failed to clone repository")
			if strings.Contains(err.Error(), "remote repository is empty") {
				err = db.SetRepositoryProcessed(repository.ID)
				if err != nil {
					log.Fatal().Err(err).Msg("could not set repository processed")
				}
			}
			_ = os.RemoveAll(dir)
			continue
		}

		var findings []report.Finding
		findings, err = FindingsForRepository(dir)
		if err != nil {
			log.Error().Err(err).Str("repository", url).Msg("could not get findings")
		} else {
			for _, f := range findings {
				if len(f.Secret) > 50 {
					f.Secret = fmt.Sprint(f.Secret[:50], "...")
				}
				outputChan <- scanct.Finding{
					Repository: repository,
					Secret:     f.Secret,
					Commit:     f.Commit,
					StartLine:  f.StartLine,
					EndLine:    f.EndLine,
					File:       f.File,
					URL:        fmt.Sprintf("%s/blob/%s/%s#L%d-%d", repository.CloneURL(), f.Commit, f.File, f.StartLine, f.EndLine),
					Rule:       f.RuleID,
					CommitDate: f.Date,
				}
			}
			err = db.SetRepositoryProcessed(repository.ID)
			if err != nil {
				log.Fatal().Err(err).Msg("could not set repository processed")
			}
		}
		err = os.RemoveAll(dir)
		if err != nil {
			log.Fatal().Err(err).Msg("could not remove clone dir")
		}
	}
}

func RepositoryOutputWorker(outputChan <-chan scanct.Finding) {
	db, err := scanct.NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not create database")
	}
	defer db.Close()
	for {
		finding, ok := <-outputChan
		if !ok {
			return
		}
		err = db.LogFinding(&finding)
		if err != nil {
			log.Fatal().Err(err).Msg("could not insert finding into database")
		}
	}
}

const RepositoryProcessWorkers = 5
const CloneRepositoryTimeout = 60 * time.Second

func ScanSecrets() {
	secretsCmd := flag.NewFlagSet("secrets", flag.ExitOnError)

	instanceFlag := secretsCmd.String("instance", "", "GitLab instance to scan")
	//tokenFlag := secretsCmd.String("token", "", "API token")
	err := secretsCmd.Parse(os.Args[2:])
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse secrets command")
	}
	if *instanceFlag == "" {
		scanct.Fan[scanct.Repository, scanct.Finding]{
			InputWorker:   RepositoryInputWorker,
			ProcessWorker: RepositoryProcessWorker,
			OutputWorker:  RepositoryOutputWorker,
			Workers:       RepositoryProcessWorkers,
			InputBuffer:   100,
			OutputBuffer:  100,
		}.Run()
	}
}
