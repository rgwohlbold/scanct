package core

import (
	"time"

	"github.com/xanzy/go-gitlab"
)

type GitLabClientWrapper struct {
	*gitlab.Client
	Token            string
	RateLimitedUntil time.Time
}

const maxPagesCount = 10
const maxItemCountPP = 100

func GetRepositories(session *Session) {
	client := session.GetClient()

	for i := 0; i < maxPagesCount; i++ {
		var lo = &gitlab.ListOptions{
			Page:    i,
			PerPage: maxItemCountPP,
		}

		var o = &gitlab.ListProjectsOptions{
			OrderBy:     gitlab.String("name"),
			ListOptions: *lo,
		}

		var projects, _, listError = client.Projects.ListProjects(o, nil)

		if listError != nil {
			session.Log.Fatal("Error: %v", listError)
			return
		}

		for _, p := range projects {
			session.Log.Debug("New Repo found >> %v\n", p.Name)
			session.Repositories <- p
		}
	}
}
