package gitlab

import (
	"context"
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
	"time"
)

type SecretsStep struct{}

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

func (s SecretsStep) UnprocessedInputs(db *scanct.Database) ([]scanct.Repository, error) {
	repositories, err := db.GetUnprocessedRepositories()
	if err != nil {
		return nil, err
	}
	rand.Shuffle(len(repositories), func(i, j int) {
		repositories[i], repositories[j] = repositories[j], repositories[i]
	})
	return repositories, nil
}

func (s SecretsStep) Process(repository *scanct.Repository) ([]scanct.Finding, error) {
	url := fmt.Sprintf("%s/%s", repository.GitLab.BaseURL, repository.Name)
	log.Debug().Str("repository", url).Msg("processing repository")

	dir := filepath.Join("/tmp", scanct.Hash(url))
	res, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal().Err(err).Msg("could not stat clone dir")
	} else if err == nil && res.IsDir() {
		err = os.RemoveAll(dir)
		if err != nil {
			log.Fatal().Err(err).Msg("could not remove existing clone dir")
		}
	}
	log.Debug().Str("repository", url).Str("temp_directory", dir).Msg("cloning repository")
	_, err = CloneRepository(repository, dir)
	defer os.RemoveAll(dir)

	if err != nil {
		return nil, errors.Wrap(err, "could not clone repository")
	}

	var findings []report.Finding
	findings, err = FindingsForRepository(dir)
	if err != nil {
		return nil, errors.Wrap(err, "could not get findings")
	}

	dbFindings := make([]scanct.Finding, len(findings))
	for _, f := range findings {
		if len(f.Secret) > 50 {
			f.Secret = fmt.Sprint(f.Secret[:50], "...")
		}
		dbFindings = append(dbFindings, scanct.Finding{
			RepositoryID: repository.ID,
			Secret:       f.Secret,
			Commit:       f.Commit,
			StartLine:    f.StartLine,
			EndLine:      f.EndLine,
			File:         f.File,
			URL:          fmt.Sprintf("%s/blob/%s/%s#L%d-%d", repository.CloneURL(), f.Commit, f.File, f.StartLine, f.EndLine),
			Rule:         f.RuleID,
			CommitDate:   f.Date,
		})
	}
	return dbFindings, nil
}

func (s SecretsStep) SetProcessed(db *scanct.Database, repository *scanct.Repository) error {
	return db.SetRepositoryProcessed(repository.ID)
}

func (s SecretsStep) SaveResult(db *scanct.Database, findings []scanct.Finding) error {
	return db.LogFindings(findings)
}

const SecretProcessWorkers = 5
const CloneRepositoryTimeout = 60 * time.Second

func ScanSecrets() {
	scanct.RunProcessStep[scanct.Repository, scanct.Finding](SecretsStep{}, SecretProcessWorkers)
}
