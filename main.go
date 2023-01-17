package main

import (
	"fmt"
	"github.com/xanzy/go-gitlab"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
)

func ProcessRepository(session *Session, url string) error {
	dir, err := GetTempDir(session, GetHash(url))
	if err != nil {
		return err
	}
	log.Debug().Str("repository", url).Str("temp_directory", dir).Msg("cloning repository")
	_, err = CloneRepository(session, url, dir)

	if err != nil {
		log.Error().Str("repository", url).Err(err).Msg("failed to clone repository")
		os.RemoveAll(dir)
		return err
	}

	checkSignatures(dir, url)
	return os.RemoveAll(dir)
}

func ProcessRepositoryWorker(session *Session) bool {
	repository, ok := <-session.Repositories
	if !ok {
		return true
	}
	log.Debug().Str("repository", repository.HTTPURLToRepo).Msg("processing repository")
	err := ProcessRepository(session, repository.HTTPURLToRepo)
	if err != nil {
		log.Error().Err(err).Str("repository", repository.HTTPURLToRepo).Msg("error processing repository")
	}
	return false
}

func ProcessRepositories(session *Session, wg *sync.WaitGroup) {
	defer wg.Done()

	var innerWg sync.WaitGroup
	threadNum := *session.Options.Threads
	innerWg.Add(threadNum)

	for i := 0; i < threadNum; i++ {
		go func() {
			defer wg.Done()
			for {
				if done := ProcessRepositoryWorker(session); done {
					return
				}
			}
		}()
	}
	innerWg.Wait()
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

	session := &Session{
		Repositories: make(chan *gitlab.Project, 1000),
	}

	var err error
	if session.Options, err = ParseOptions(); err != nil {
		log.Fatal().Err(err).Msg("could not parse options")
	}

	if session.Config, err = ParseConfig(); err != nil {
		log.Fatal().Err(err).Msg("could not parse config")
	}
	rand.Seed(time.Now().Unix())

	session.InitLogger()
	session.InitThreads()
	session.InitGitLabClients()
	log.Debug().Int("worker_threads", *session.Options.Threads).Str("temp_directory", *session.Options.TempDirectory).Msg("starting shhgit")

	wg.Add(2)
	go GetRepositories(session, &wg)
	go ProcessRepositories(session, &wg)

	wg.Wait()
}
