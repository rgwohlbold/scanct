package main

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const GitlabMagicURL = "/users/sign_in"
const GitlabMagicString = "<meta content=\"GitLab\" property=\"og:site_name\">"
const GitlabRegisterMagicString = "<a data-qa-selector=\"register_link\" href=\"/users/sign_up\">Register now</a>"

type GitlabFilter struct{}

func (g GitlabFilter) UnprocessedInstances(db *Database) ([]Instance, error) {
	return db.GetUnprocessedInstancesForGitlab()
}

func (g GitlabFilter) ProcessInstance(instance *Instance) (GitLab, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(fmt.Sprintf("https://%s%s", instance.Name, GitlabMagicURL))
	if err != nil {
		return GitLab{}, errors.Wrap(err, "error requesting instance")
	} else if resp.StatusCode != 200 {
		return GitLab{}, errors.New(fmt.Sprintf("no instance found: status %d", resp.StatusCode))
	} else {
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return GitLab{}, err
		}
		bodyStr := string(body)
		if strings.Contains(bodyStr, GitlabMagicString) {
			return GitLab{
				InstanceID:  instance.ID,
				AllowSignup: strings.Contains(bodyStr, GitlabRegisterMagicString),
				Email:       "",
				Password:    "",
				APIToken:    "",
				Processed:   false,
				BaseURL:     fmt.Sprintf("https://%s", instance.Name),
			}, nil
		}
	}
	return GitLab{}, errors.New("no instance found: no magic string")
}

func (g GitlabFilter) SaveResult(db *Database, result GitLab) error {
	return db.AddGitLab(result)
}

func RunFilterGitlabCommand() {
	RunFilter[GitLab](GitlabFilter{}, 5)
}
