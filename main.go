package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
	"gitlab.hpi.de/lukas.radermacher/shhgit-for-gitlab/core"
)

type MatchEvent struct {
	Url       string
	Matches   []string
	Signature string
	File      string
	Stars     int
	Source    core.GitResourceType
}

var session = core.GetSession()

func ProcessRepositories(wg *sync.WaitGroup) {
	defer wg.Done()

	var innerWg sync.WaitGroup
	threadNum := *session.Options.Threads
	innerWg.Add(threadNum)

	for i := 0; i < threadNum; i++ {
		go func(tid int) {
			defer innerWg.Done()

			for {
				timeout := time.Duration(*session.Options.CloneRepositoryTimeout) * time.Second
				_, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				repository, open := <-session.Repositories

				if !open {
					break
				}

				log.Debug().Str("repository", repository.HTTPURLToRepo).Msg("processing repository")
				processRepositoryOrGist(repository.HTTPURLToRepo, repository.DefaultBranch, repository.StarCount, core.GITHUB_SOURCE)
			}
		}(i)
	}

	innerWg.Wait()
}

func ProcessGists() {
	threadNum := *session.Options.Threads

	for i := 0; i < threadNum; i++ {
		go func(tid int) {
			for {
				gistUrl := <-session.Gists
				processRepositoryOrGist(gistUrl, "", -1, core.GIST_SOURCE)
			}
		}(i)
	}
}

func ProcessComments() {
	threadNum := *session.Options.Threads

	for i := 0; i < threadNum; i++ {
		go func(tid int) {
			for {
				commentBody := <-session.Comments
				dir := core.GetTempDir(core.GetHash(commentBody))
				ioutil.WriteFile(filepath.Join(dir, "comment.ignore"), []byte(commentBody), 0644)

				if !checkSignatures(dir, "ISSUE", 0, core.GITHUB_COMMENT) {
					os.RemoveAll(dir)
				}
			}
		}(i)
	}
}

func processRepositoryOrGist(url string, ref string, stars int, source core.GitResourceType) {
	var (
		matchedAny bool = false
	)

	dir := core.GetTempDir(core.GetHash(url))
	log.Debug().Str("repository", url).Str("temp_directory", dir).Msg("cloning repository")
	_, err := core.CloneRepository(session, url, ref, dir)

	if err != nil {
		log.Error().Str("repository", url).Err(err).Msg("failed to clone repository")
		os.RemoveAll(dir)
		return
	}

	matchedAny = checkSignatures(dir, url, stars, source)
	if !matchedAny {
		os.RemoveAll(dir)
	}
}

func printFinding(url string, f report.Finding) {
	const maxLength = 50
	event := log.Warn()
	if len(f.Secret) > 50 {
		event.Str("secret", fmt.Sprintf("%s...", f.Secret[:maxLength]))
	} else {
		event.Str("secret", f.Secret)
	}
	event.Str("file", f.File).Str("url", url).Str("commit", f.Commit).Int("startLine", f.StartLine).Int("endLine", f.StartLine).Str("rule", f.RuleID).Msg("potential leak")
}

func checkSignatures(dir string, url string, stars int, source core.GitResourceType) (matchedAny bool) {
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create detector")
	}
	findings, err := detector.DetectGit(dir, "", detect.DetectType)
	if err != nil {
		log.Error().Err(err).Msg("failed to create detector")
		return
	}
	for _, finding := range findings {
		printFinding(url, finding)
	}

	return len(findings) != 0
}

func main() {
	log.Debug().Int("worker_threads", *session.Options.Threads).Str("temp_directory", *session.Options.TempDirectory).Msg("starting shhgit")

	if len(*session.Options.Local) > 0 {
		log.Info().Str("directory", *session.Options.Local).Msg("scanning local directory")
		rc := 0
		if checkSignatures(*session.Options.Local, *session.Options.Local, -1, core.LOCAL_SOURCE) {
			rc = 1
		} else {
			log.Info().Str("directory", *session.Options.Local).Msg("no leaks found")
		}
		os.Exit(rc)
	} else {
		var wg sync.WaitGroup

		if *session.Options.SearchQuery != "" {
			log.Warn().Str("query", *session.Options.SearchQuery).Msg("searching for repositories")
		}

		wg.Add(2)
		go core.GetRepositories(session, &wg)
		go ProcessRepositories(&wg)

		wg.Wait()
	}
}
