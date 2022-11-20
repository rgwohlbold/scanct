package core

import (
	"sync"
	"time"

	"github.com/xanzy/go-gitlab"
)

type GitLabClientWrapper struct {
	*gitlab.Client
	Token            string
	RateLimitedUntil time.Time
}

const maxPagesCount = -1   // -1: no limit
const maxItemCountPP = 100 // 100 is the maximum defined by the GitLab API

func GetRepositories(session *Session, wg *sync.WaitGroup) {
	defer wg.Done()

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
			session.Log.Fatal("Error: %v", listError)
			return
		}

		for _, p := range projects {
			session.Log.Debug("New Repo found >> %v\n", p.Name)
			session.Repositories <- p
		}

		page = res.NextPage
	}

	close(session.Repositories)
}
