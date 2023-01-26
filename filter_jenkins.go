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

func (g JenkinsFilter) UnprocessedInstances(db *Database) ([]Instance, error) {
	return db.GetUnprocessedInstancesForJenkins()
}

func (g JenkinsFilter) ProcessInstance(instance *Instance) (Jenkins, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(fmt.Sprintf("https://%s%s", instance.Name, JenkinsMagicURL))
	if err != nil {
		return Jenkins{}, errors.Wrap(err, "error requesting instance")
	} else if resp.StatusCode != 200 {
		return Jenkins{}, errors.New(fmt.Sprintf("no instance found: status %d", resp.StatusCode))
	} else {
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return Jenkins{}, err
		}
		return Jenkins{
			InstanceID:   instance.ID,
			AnonymousAPI: len(string(body)) > 2 && resp.Header.Get("x-jenkins") != "",
			BaseURL:      fmt.Sprintf("https://%s", instance.Name),
		}, nil
	}
}

func (g JenkinsFilter) SaveResult(db *Database, result Jenkins) error {
	return db.AddJenkins(result)
}

func RunFilterJenkinsCommand() {
	RunFilter[Jenkins](JenkinsFilter{}, 5)
}
