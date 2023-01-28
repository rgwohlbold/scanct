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

func (g GitlabFilter) SetProcessed(db *Database, i *Instance) error {
	return db.SetGitlabProcessed(i.ID)
}

func (g GitlabFilter) UnprocessedInstances(db *Database) ([]Instance, error) {
	return db.GetUnprocessedInstancesForGitlab()
}

func (g GitlabFilter) ProcessInstance(instance *Instance) ([]GitLab, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(fmt.Sprintf("https://%s%s", instance.Name, GitlabMagicURL))
	if err != nil {
		return nil, errors.Wrap(err, "error requesting instance")
	} else if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("no instance found: status %d", resp.StatusCode))
	} else {
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		bodyStr := string(body)
		if strings.Contains(bodyStr, GitlabMagicString) {
			return []GitLab{{
				InstanceID:  instance.ID,
				AllowSignup: strings.Contains(bodyStr, GitlabRegisterMagicString),
				Email:       "",
				Password:    "",
				APIToken:    "",
				Processed:   false,
				BaseURL:     fmt.Sprintf("https://%s", instance.Name),
			}}, nil
		}
	}
	return nil, errors.New("no instance found: no magic string")
}

func (g GitlabFilter) SaveResult(db *Database, result []GitLab) error {
	for _, r := range result {
		err := db.AddGitLab(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func RunFilterGitlabCommand() {
	RunFilter[Instance, GitLab](GitlabFilter{}, 5)
}
