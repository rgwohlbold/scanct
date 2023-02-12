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
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"math/rand"
	"runtime"
	"time"
)

type SecretsStep struct{}

func CloneRepository(r *scanct.Repository) (*git.Repository, error) {
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

	repository, err := git.CloneContext(localCtx, memory.NewStorage(), nil, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to clone repository")
	}
	return repository, nil
}

func FindingsForRepository(repository *git.Repository) ([]report.Finding, error) {
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create detector")
	}
	it, err := repository.Log(&git.LogOptions{
		All: true,
	})
	secretsMap := make(map[string]struct{})
	var findings []report.Finding
	err = it.ForEach(func(c *object.Commit) error {
		var fileIt *object.FileIter
		fileIt, err = c.Files()
		if err != nil {
			return err
		}
		return fileIt.ForEach(func(file *object.File) error {
			var contents string
			contents, err = file.Contents()
			if err != nil {
				return err
			}
			for _, finding := range detector.Detect(detect.Fragment{
				Raw:       contents,
				FilePath:  file.Name,
				CommitSHA: c.Hash.String(),
			}) {
				if _, ok := secretsMap[finding.Secret]; !ok {
					secretsMap[finding.Secret] = struct{}{}
					finding.Commit = c.Hash.String()
					finding.Date = c.Author.When.Format("2006-01-02")
					findings = append(findings, finding)
				}
			}
			return nil
		})
	})
	return findings, err
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
	log.Debug().Str("repository", url).Msg("cloning repository")
	r, err := CloneRepository(repository)

	if err != nil {
		return nil, errors.Wrap(err, "could not clone repository")
	}
	log.Debug().Str("repository", url).Msg("processing repository")
	var findings []report.Finding
	findings, err = FindingsForRepository(r)
	if err != nil {
		return nil, errors.Wrap(err, "could not get findings")
	}
	log.Debug().Str("repository", url).Int("findings", len(findings)).Msg("done processing")

	dbFindings := make([]scanct.Finding, 0, len(findings))
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
	return db.SetRepositoryProcessed(repository)
}

func (s SecretsStep) SaveResult(db *scanct.Database, findings []scanct.Finding) error {
	return db.LogFindings(findings)
}

const CloneRepositoryTimeout = 60 * time.Second

func ScanSecrets() {
	scanct.RunProcessStep[scanct.Repository, scanct.Finding](SecretsStep{}, runtime.NumCPU())
}
