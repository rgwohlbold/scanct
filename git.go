package main

import (
	"context"
	"github.com/pkg/errors"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

func CloneRepository(session *Session, url string, dir string) (*git.Repository, error) {
	timeout := time.Duration(*session.Options.CloneRepositoryTimeout) * time.Second
	localCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opts := &git.CloneOptions{
		Depth:             1,
		RecurseSubmodules: git.NoRecurseSubmodules,
		URL:               url,
		SingleBranch:      false,
		Tags:              git.NoTags,
	}
	if session.Config.GitLabApiToken != "" {
		opts.Auth = &http.BasicAuth{Username: "git", Password: session.Config.GitLabApiToken}
	}

	repository, err := git.PlainCloneContext(localCtx, dir, false, opts)

	if err != nil {
		return nil, errors.Wrap(err, "failed to clone repository")
	}

	return repository, nil
}
