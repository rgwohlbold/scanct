package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"github.com/rs/zerolog"
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

				// if uint(repository.StarCount) >= *session.Options.MinimumStars &&
				// 	uint(repository.Statistics.RepositorySize) < *session.Options.MaximumRepositorySize {
				session.Log.Debug("Processing Repository %v", repository.Name)
				processRepositoryOrGist(repository.HTTPURLToRepo, repository.DefaultBranch, repository.StarCount, core.GITHUB_SOURCE)
				// }
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
	_, err := core.CloneRepository(session, url, ref, dir)

	if err != nil {
		session.Log.Debug("[%s] Cloning failed: %s", url, err.Error())
		os.RemoveAll(dir)
		return
	}

	session.Log.Debug("[%s] Cloning %s in to %s", url, ref, strings.Replace(dir, *session.Options.TempDirectory, "", -1))
	matchedAny = checkSignatures(dir, url, stars, source)
	if !matchedAny {
		os.RemoveAll(dir)
	}
}

func printFinding(f report.Finding) {
	// trim all whitespace and tabs from the line
	f.Line = strings.TrimSpace(f.Line)
	// trim all whitespace and tabs from the secret
	f.Secret = strings.TrimSpace(f.Secret)
	// trim all whitespace and tabs from the match
	f.Match = strings.TrimSpace(f.Match)

	matchInLineIDX := strings.Index(f.Line, f.Match)
	secretInMatchIdx := strings.Index(f.Match, f.Secret)

	start := f.Line[0:matchInLineIDX]
	startMatchIdx := 0
	if matchInLineIDX > 20 {
		startMatchIdx = matchInLineIDX - 20
		start = "..." + f.Line[startMatchIdx:matchInLineIDX]
	}

	matchBeginning := lipgloss.NewStyle().SetString(f.Match[0:secretInMatchIdx]).Foreground(lipgloss.Color("#f5d445"))
	secret := lipgloss.NewStyle().SetString(f.Secret).
		Bold(true).
		Italic(true).
		Foreground(lipgloss.Color("#f05c07"))
	matchEnd := lipgloss.NewStyle().SetString(f.Match[secretInMatchIdx+len(f.Secret):]).Foreground(lipgloss.Color("#f5d445"))
	lineEnd := f.Line[matchInLineIDX+len(f.Match):]
	if len(f.Secret) > 100 {
		secret = lipgloss.NewStyle().SetString(f.Secret[0:100] + "...").
			Bold(true).
			Italic(true).
			Foreground(lipgloss.Color("#f05c07"))
	}
	if len(lineEnd) > 20 {
		lineEnd = lineEnd[0:20] + "..."
	}

	finding := fmt.Sprintf("%s%s%s%s%s\n", strings.TrimPrefix(strings.TrimLeft(start, " "), "\n"), matchBeginning, secret, matchEnd, lineEnd)
	fmt.Printf("%-12s %s", "Finding:", finding)
	fmt.Printf("%-12s %s\n", "Secret:", secret)
	fmt.Printf("%-12s %s\n", "RuleID:", f.RuleID)
	fmt.Printf("%-12s %f\n", "Entropy:", f.Entropy)
	if f.File == "" {
		fmt.Println("")
		return
	}
	fmt.Printf("%-12s %s\n", "File:", f.File)
	fmt.Printf("%-12s %d\n", "Line:", f.StartLine)
	if f.Commit == "" {
		fmt.Printf("%-12s %s\n", "Fingerprint:", f.Fingerprint)
		fmt.Println("")
		return
	}
	fmt.Printf("%-12s %s\n", "Commit:", f.Commit)
	fmt.Printf("%-12s %s\n", "Author:", f.Author)
	fmt.Printf("%-12s %s\n", "Email:", f.Email)
	fmt.Printf("%-12s %s\n", "Date:", f.Date)
	fmt.Printf("%-12s %s\n", "Fingerprint:", f.Fingerprint)
	fmt.Println("")
}

func checkSignatures(dir string, url string, stars int, source core.GitResourceType) (matchedAny bool) {
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		session.Log.Error("Error while creating detector: %s", err.Error())
		os.Exit(1)
	}
	findings, err := detector.DetectGit(dir, "", detect.DetectType)
	if err != nil {
		session.Log.Error("Error while detecting files: %s", err.Error())
		return
	}
	for _, finding := range findings {
		printFinding(finding)
	}

	return len(findings) != 0
}

func publish(event *MatchEvent) {
	// todo: implement a modular plugin system to handle the various outputs (console, live, csv, webhooks, etc)
	if len(*session.Options.Live) > 0 {
		data, _ := json.Marshal(event)
		http.Post(*session.Options.Live, "application/json", bytes.NewBuffer(data))
	}
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	session.Log.Info(color.HiBlueString(core.Banner))
	session.Log.Info("\t%s\n", color.HiCyanString(core.Author))
	session.Log.Info("[*] Loaded %s signatures. Using %s worker threads. Temp work dir: %s\n", color.BlueString("%d", len(session.Signatures)), color.BlueString("%d", *session.Options.Threads), color.BlueString(*session.Options.TempDirectory))

	if len(*session.Options.Local) > 0 {
		session.Log.Info("[*] Scanning local directory: %s - skipping public repository checks...", color.BlueString(*session.Options.Local))
		rc := 0
		if checkSignatures(*session.Options.Local, *session.Options.Local, -1, core.LOCAL_SOURCE) {
			rc = 1
		} else {
			session.Log.Info("[*] No matching secrets found in %s!", color.BlueString(*session.Options.Local))
		}
		os.Exit(rc)
	} else {
		var wg sync.WaitGroup

		if *session.Options.SearchQuery != "" {
			session.Log.Important("Search Query '%s' given. Only returning matching results.", *session.Options.SearchQuery)
		}

		wg.Add(2)
		go core.GetRepositories(session, &wg)
		go ProcessRepositories(&wg)
		// go ProcessComments()

		// if *session.Options.ProcessGists {
		// 	go core.GetGists(session)
		// 	go ProcessGists()
		// }

		spinny := core.ShowSpinner()
		wg.Wait()
		spinny()
	}
}
