package main

import (
	"context"
	"fmt"
	"github.com/xanzy/go-gitlab"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
	"gitlab.platypwnies.de/cybersecurity-klub-hpi/shhgit/core"
)

func ProcessRepositories(session *core.Session, wg *sync.WaitGroup) {
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
				err := processRepositoryOrGist(session, repository.HTTPURLToRepo)
				if err != nil {
					return
				}
			}
		}(i)
	}

	innerWg.Wait()
}

func processRepositoryOrGist(session *core.Session, url string) error {
	dir := core.GetTempDir(session, core.GetHash(url))
	log.Debug().Str("repository", url).Str("temp_directory", dir).Msg("cloning repository")
	_, err := core.CloneRepository(session, url, dir)

	if err != nil {
		log.Error().Str("repository", url).Err(err).Msg("failed to clone repository")
		os.RemoveAll(dir)
		return err
	}

	checkSignatures(dir, url)
	return os.RemoveAll(dir)
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

func checkSignatures(dir string, url string) (matchedAny bool) {
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create detector")
	}
	findings, err := detector.DetectGit(dir, "", detect.DetectType)
	if err != nil {
		log.Error().Err(err).Msg("failed to create detector")
		return
	}
	secrets := make([]string, 0)
	for _, finding := range findings {
		found := false
		for _, secret := range secrets {
			if secret == finding.Secret {
				found = true
				break
			}
		}
		if !found {
			secrets = append(secrets, finding.Secret)
			printFinding(url, finding)
		} else {
			log.Debug().Str("repository", url).Msg("duplicate secret found")
		}
	}

	return len(findings) != 0
}

func main() {
	var wg sync.WaitGroup

	session := &core.Session{
		Repositories: make(chan *gitlab.Project, 1000),
	}

	var err error
	if session.Options, err = core.ParseOptions(); err != nil {
		log.Fatal().Err(err).Msg("could not parse options")
	}

	if session.Config, err = core.ParseConfig(); err != nil {
		log.Fatal().Err(err).Msg("could not parse config")
	}
	rand.Seed(time.Now().Unix())

	session.InitLogger()
	session.InitThreads()
	session.InitGitLabClients()
	log.Debug().Int("worker_threads", *session.Options.Threads).Str("temp_directory", *session.Options.TempDirectory).Msg("starting shhgit")

	wg.Add(2)
	go core.GetRepositories(session, &wg)
	go ProcessRepositories(session, &wg)

	wg.Wait()
}
