package gitlab

import (
	"github.com/pkg/errors"
	"github.com/rgwohlbold/scanct"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

func ProcessGitlab(db *scanct.Database, gl *scanct.GitLab) error {
	client, err := gitlab.NewClient(gl.APIToken, gitlab.WithBaseURL(gl.URL()))
	if err != nil {
		return errors.Wrap(err, "could not create gitlab client")
	}
	var projects []*gitlab.Project
	var res *gitlab.Response

	for page := 1; page != 0; {
		options := &gitlab.ListProjectsOptions{
			OrderBy: gitlab.String("name"),
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: MaxItemCountPerPage,
			}}

		projects, res, err = client.Projects.ListProjects(options, nil)
		if err != nil {
			return errors.Wrap(err, "failed to list")
		}
		err = db.InsertProjects(gl, projects)
		if err != nil {
			return errors.Wrap(err, "failed to insert projects")
		}
		page = res.NextPage
	}
	return db.SetGitlabProcessed(gl.ID)
}

func ImportRepositories() {
	db, err := scanct.NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("could not open database")
	}
	defer db.Close()

	gitlabs, err := db.GetUnprocessedGitLabs()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get gitlabs")
	}

	for _, gl := range gitlabs {
		log.Debug().Str("gitlab", gl.URL()).Msg("processing gitlab")
		err = ProcessGitlab(&db, &gl)
		if err != nil {
			log.Error().Err(err).Msg("failed to process gitlab")
		}
	}
}
