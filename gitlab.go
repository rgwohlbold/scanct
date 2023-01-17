package main

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

type GitLabClientWrapper struct {
	*gitlab.Client
	Token            string
	RateLimitedUntil time.Time
}

const maxPagesCount = -1   // -1: no limit
const maxItemCountPP = 100 // 100 is the maximum defined by the GitLab API

func GetRepositories(session *Session) {
	client := session.GetClient()

	for page := 1; page != 0 && (maxPagesCount == -1 || page <= maxPagesCount); {
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
			log.Fatal().Err(listError).Msg("failed to list")
			return
		}

		for _, p := range projects {
			log.Debug().Str("repository", p.HTTPURLToRepo).Msg("new repository")
			session.Repositories <- p
		}

		page = res.NextPage
	}

	close(session.Repositories)
}
