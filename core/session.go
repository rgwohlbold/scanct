package core

import (
	"context"
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

type Session struct {
	sync.Mutex

	Version          string
	Options          *Options
	Config           *Config
	Repositories     chan *gitlab.Project
	Comments         chan string
	Context          context.Context
	Clients          chan *GitLabClientWrapper
	ExhaustedClients chan *GitLabClientWrapper
	CsvWriter        *csv.Writer
}

var (
	session     *Session
	sessionSync sync.Once
)

func (s *Session) Start() {
	rand.Seed(time.Now().Unix())

	s.InitLogger()
	s.InitThreads()
	s.InitGitLabClients()
}

func (s *Session) InitLogger() {
	if *s.Options.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}
}

func (s *Session) InitGitLabClients() {
	s.Clients = make(chan *GitLabClientWrapper, 1)

	client, err := gitlab.NewClient(s.Config.GitLabApiToken, gitlab.WithBaseURL(s.Config.GitLabApiEndpoint))

	if err != nil {
		log.Fatal().Err(err).Msg("could not create GitLab client")
	}

	s.Clients <- &GitLabClientWrapper{client, s.Config.GitLabApiToken, time.Now().Add(-1 * time.Second)}
}

func (s *Session) GetClient() *GitLabClientWrapper {
	for {
		select {

		case client := <-s.Clients:
			//s.Log.Debug("Using client with token: %s", client.Token)
			return client

		case client := <-s.ExhaustedClients:
			sleepTime := time.Until(client.RateLimitedUntil)
			log.Warn().Dur("sleepTime", sleepTime).Msg("rate limited, sleeping")
			time.Sleep(sleepTime)
			log.Debug().Msg("returning client to pool")
			s.FreeClient(client)

		default:
			log.Debug().Int("available_clients", len(s.Clients)).Int("exhaustedClients", len(s.ExhaustedClients)).Msg("no clients available, sleeping")
			time.Sleep(time.Millisecond * 1000)
		}
	}
}

// FreeClient returns the GitLab Client to the pool of available,
// non-rate-limited channel of clients in the session
func (s *Session) FreeClient(client *GitLabClientWrapper) {
	if client.RateLimitedUntil.After(time.Now()) {
		s.ExhaustedClients <- client
	} else {
		s.Clients <- client
	}
}

func (s *Session) InitThreads() {
	if *s.Options.Threads == 0 {
		numCPUs := runtime.NumCPU()
		s.Options.Threads = &numCPUs
	}

	runtime.GOMAXPROCS(*s.Options.Threads + 1)
}

func GetSession() *Session {
	sessionSync.Do(func() {
		session = &Session{
			Context:      context.Background(),
			Repositories: make(chan *gitlab.Project, 1000),
			Comments:     make(chan string, 1000),
		}

		var err error
		if session.Options, err = ParseOptions(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if session.Config, err = ParseConfig(session.Options); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		session.Start()
	})

	return session
}
