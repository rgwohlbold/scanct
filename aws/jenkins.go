package aws

import (
	"fmt"
	"github.com/rgwohlbold/scanct"
	"io"
	"net/http"
	"strings"
	"time"
)

type JenkinsStep struct{}

func (g JenkinsStep) SetProcessed(db *scanct.Database, f *scanct.JenkinsFinding) error {
	return db.SetJenkinsFindingProcessed(f.ID)
}

func (g JenkinsStep) UnprocessedInputs(db *scanct.Database) ([]scanct.JenkinsFinding, error) {
	return db.GetUnprocessedJenkinsAWSFindings()
}

func (g JenkinsStep) Process(finding *scanct.JenkinsFinding) ([]scanct.AWSKey, error) {
	client := http.Client{Timeout: 5 * time.Second}

	finding.File = finding.File[strings.Index(finding.File, "/")+1:]
	finding.File = finding.File[strings.Index(finding.File, "/")+1:]
	finding.File = finding.File[strings.Index(finding.File, "/")+1:]

	url := finding.Job.URL + "/ws/" + finding.File
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got status %d", resp.StatusCode)
	}
	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	key, arn, err := ParseSecret(finding.Secret, string(body))
	if err != nil {
		return nil, err
	}
	return []scanct.AWSKey{{
		AccessKey: finding.Secret,
		SecretKey: key,
		FindingID: finding.ID,
		Arn:       arn,
	}}, nil
}

func (g JenkinsStep) SaveResult(db *scanct.Database, result []scanct.AWSKey) error {
	return db.AddAWSKeys(result)
}

func RunJenkinsKeysStep() {
	scanct.RunProcessStep[scanct.JenkinsFinding, scanct.AWSKey](JenkinsStep{}, 5)
}
