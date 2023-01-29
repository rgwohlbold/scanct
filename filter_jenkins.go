package main

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"time"
)

const JenkinsMagicURL = "/api/json"

type JenkinsFilter struct{}

func (g JenkinsFilter) SetProcessed(db *Database, i *Instance) error {
	return db.SetInstanceProcessed(i.ID)
}

func (g JenkinsFilter) UnprocessedInstances(db *Database) ([]Instance, error) {
	return db.GetUnprocessedInstancesForJenkins()
}

func (g JenkinsFilter) ProcessInstance(instance *Instance) ([]Jenkins, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(fmt.Sprintf("https://%s%s", instance.Name, JenkinsMagicURL))
	if err != nil {
		return nil, errors.Wrap(err, "error requesting instance")
	} else if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("no instance found: status %d", resp.StatusCode))
	} else {
		scriptAccess := false

		var resp2 *http.Response
		resp2, err = client.Get(fmt.Sprintf("https://%s/script", instance.Name))
		if err == nil && resp2.StatusCode == 200 {
			scriptAccess = true
		}

		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.Header.Get("x-jenkins") != "" {
			return []Jenkins{{
				InstanceID:   instance.ID,
				AnonymousAPI: len(string(body)) > 2,
				BaseURL:      fmt.Sprintf("https://%s", instance.Name),
				ScriptAccess: scriptAccess,
			}}, nil
		}
	}
	return nil, nil
}

func (g JenkinsFilter) SaveResult(db *Database, result []Jenkins) error {
	return db.AddJenkins(result)
}

func RunFilterJenkinsCommand() {
	RunFilter[Instance, Jenkins](JenkinsFilter{}, 5)
}
