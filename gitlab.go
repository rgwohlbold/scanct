package main

import (
	"context"
	"flag"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"os"
	"time"
)

const maxItemCountPP = 100 // 100 is the maximum defined by the GitLab API

//func (g *GitlabInstance) AddToDBIfNotExists() error {
//	db, err := NewDatabase()
//	defer db.Close()
//	if err != nil {
//		return errors.Wrap(err, "could not open database")
//	}
//	err = db.AddGitlabIfNotExists(g)
//	if err != nil {
//		return errors.Wrap(err, "could not add gitlab to database")
//	}
//	return nil
//}

type GitFinding struct {
	report.Finding
	GitLab     *GitLab
	Repository *Repository
}

type ScanRepositoriesConfig struct {
	GitLab         *GitLab
	TempDirectory  string
	GitLabApiToken string
}

func CloneRepository(config *ScanRepositoriesConfig, url string, dir string) (*git.Repository, error) {
	localCtx, cancel := context.WithTimeout(context.Background(), CloneRepositoryTimeout)
	defer cancel()

	opts := &git.CloneOptions{
		Depth:             1,
		RecurseSubmodules: git.NoRecurseSubmodules,
		URL:               url,
		SingleBranch:      false,
		Tags:              git.NoTags,
	}
	if config.GitLabApiToken != "" {
		opts.Auth = &http.BasicAuth{Username: "git", Password: config.GitLabApiToken}
	}

	repository, err := git.PlainCloneContext(localCtx, dir, false, opts)

	if err != nil {
		return nil, errors.Wrap(err, "failed to clone repository")
	}

	return repository, nil
}

func RepositoryInputWorker(config *ScanRepositoriesConfig, inputChan chan<- *gitlab.Project) {
	defer close(inputChan)

	client, err := gitlab.NewClient(config.GitLabApiToken, gitlab.WithBaseURL(config.GitLab.URL()))
	if err != nil {
		log.Fatal().Err(err).Msg("could not create gitlab client")
	}

	for page := 1; page != 0; {
		var lo = &gitlab.ListOptions{
			Page:    page,
			PerPage: maxItemCountPP,
		}

		var o = &gitlab.ListProjectsOptions{
			OrderBy:     gitlab.String("name"),
			ListOptions: *lo,
		}

		var projects, res, listError = client.Projects.ListProjects(o, nil)

		if listError != nil {
			log.Error().Err(listError).Msg("failed to list")
			return
		}

		for _, p := range projects {
			inputChan <- p
		}
		page = res.NextPage
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
	_findings := make([]report.Finding, len(findings))
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

func RepositoryProcessWorker(config *ScanRepositoriesConfig, inputChan <-chan *gitlab.Project, outputChan chan<- GitFinding) {
	for {
		repository, ok := <-inputChan
		if !ok {
			return
		}
		url := repository.HTTPURLToRepo
		log.Debug().Str("repository", url).Msg("processing repository")

		dir, err := GetTempDir(GetHash(url))
		if err != nil {
			log.Fatal().Err(err).Msg("could not get temp dir")
		}
		log.Debug().Str("repository", url).Str("temp_directory", dir).Msg("cloning repository")
		_, err = CloneRepository(config, url, dir)

		if err != nil {
			log.Warn().Str("repository", url).Err(err).Msg("failed to clone repository")
			_ = os.RemoveAll(dir)
			return
		}

		findings, err := FindingsForRepository(dir)
		if err != nil {
			log.Fatal().Err(err).Str("repository", url).Msg("could not get findings")
		}
		for _, f := range findings {
			outputChan <- GitFinding{
				GitLab:     config.GitLab,
				Finding:    f,
				Repository: nil,
			}
			_ = os.RemoveAll(dir)
		}
	}
}

func RepositoryOutputWorker(outputChan <-chan GitFinding) {
	db, err := NewDatabase()
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

//func printFinding(url string, f report.GitFinding) {
//	const maxLength = 50
//	event := log.Warn()
//	if len(f.Secret) > 50 {
//		event.Str("secret", fmt.Sprintf("%s...", f.Secret[:maxLength]))
//	} else {
//		event.Str("secret", f.Secret)
//	}
//	event.Str("file", f.File).Str("url", url).Str("commit", f.Commit).Int("startLine", f.StartLine).Int("endLine", f.StartLine).Str("rule", f.RuleID).Msg("potential leak")
//}

const RepositoryProcessWorkers = 5
const CloneRepositoryTimeout = 60 * time.Second

func ScanRepositories(config *ScanRepositoriesConfig) {
	Fan[*gitlab.Project, GitFinding]{
		InputWorker: func(inputChan chan<- *gitlab.Project) { RepositoryInputWorker(config, inputChan) },
		ProcessWorker: func(inputChan <-chan *gitlab.Project, outputChan chan<- GitFinding) {
			RepositoryProcessWorker(config, inputChan, outputChan)
		},
		OutputWorker: RepositoryOutputWorker,
		Workers:      RepositoryProcessWorkers,
		InputBuffer:  100,
		OutputBuffer: 100,
	}.Run()
}

func RunSecrets() {
	secretsCmd := flag.NewFlagSet("secrets", flag.ExitOnError)

	instanceFlag := secretsCmd.String("instance", "", "GitLab instance to scan")
	//tokenFlag := secretsCmd.String("token", "", "API token")
	err := secretsCmd.Parse(os.Args[2:])
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse secrets command")
	}
	if *instanceFlag == "" {
		var db Database
		db, err = NewDatabase()
		defer db.Close()
		if err != nil {
			log.Fatal().Err(err).Msg("could not create database")
		}
		var gitlabs []GitLab
		gitlabs, err = db.GetUnprocessedGitLabs()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get unprocessed gitlabs")
		}
		for i := range gitlabs {
			log.Debug().Str("gitlab", gitlabs[i].Instance.Name).Msg("processing gitlab")
			ScanRepositories(&ScanRepositoriesConfig{
				GitLab:         &gitlabs[i],
				TempDirectory:  "/tmp",
				GitLabApiToken: "",
			})
			err = db.SetGitlabProcessed(gitlabs[i].ID)
			if err != nil {
				log.Error().Err(err).Msg("could not set gitlab as processed")
			}
		}
	}
	//return SecretsCommand, &ScanRepositoriesConfig{
	//	Instance: &GitlabInstance{
	//		GitlabID: -1,
	//		Domain:   *instanceFlag,
	//	},
	//	TempDirectory:  "/tmp",
	//	GitLabApiToken: *tokenFlag,
	//}, err
}
