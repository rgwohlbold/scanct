package gitlab

import (
	"github.com/pkg/errors"
	"github.com/rgwohlbold/scanct"
	"github.com/xanzy/go-gitlab"
)

const MaxItemCountPerPage = 100 // 100 is the maximum defined by the GitLab API

type RepositoryStep struct{}

func (r RepositoryStep) UnprocessedInputs(db *scanct.Database) ([]scanct.GitLab, error) {
	return db.GetUnprocessedGitLabs()
}

func (r RepositoryStep) Process(gl *scanct.GitLab) ([]scanct.Repository, error) {
	client, err := gitlab.NewClient(gl.APIToken, gitlab.WithBaseURL(gl.URL()))
	if err != nil {
		return nil, errors.Wrap(err, "could not create gitlab client")
	}
	var res *gitlab.Response
	repositories := make([]scanct.Repository, 0)

	for page := 1; page != 0; page = res.NextPage {
		options := &gitlab.ListProjectsOptions{
			OrderBy: gitlab.String("name"),
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: MaxItemCountPerPage,
			}}

		var projects []*gitlab.Project
		projects, res, err = client.Projects.ListProjects(options, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list")
		}
		for _, project := range projects {
			repositories = append(repositories, scanct.Repository{
				GitLabID:  gl.ID,
				Name:      project.PathWithNamespace,
				Processed: false,
			})
		}
	}
	return repositories, nil
}

func (r RepositoryStep) SetProcessed(db *scanct.Database, i *scanct.GitLab) error {
	return db.SetGitlabProcessed(i.ID)
}

func (r RepositoryStep) SaveResult(db *scanct.Database, repos []scanct.Repository) error {
	return db.InsertRepositories(repos)
}

func ImportRepositories() {
	scanct.RunProcessStep[scanct.GitLab, scanct.Repository](RepositoryStep{}, 5)
}
