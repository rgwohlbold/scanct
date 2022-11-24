package core

import (
	"context"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

type GitResourceType int

const (
	LOCAL_SOURCE GitResourceType = iota
	GITHUB_SOURCE
	GITHUB_COMMENT
	GIST_SOURCE
	BITBUCKET_SOURCE
	GITLAB_SOURCE
)

type GitResource struct {
	Id   int64
	Type GitResourceType
	Url  string
	Ref  string
}

func CloneRepository(session *Session, url string, ref string, dir string) (*git.Repository, error) {
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
		return nil, err
	}

	return repository, nil
}
