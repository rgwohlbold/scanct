package aws

import (
	"fmt"
	"github.com/rgwohlbold/scanct"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"time"
)

type GitlabStep struct{}

func (g GitlabStep) SetProcessed(db *scanct.Database, f *scanct.Finding) error {
	return db.SetFindingProcessed(f.ID)
}

func (g GitlabStep) UnprocessedInputs(db *scanct.Database) ([]scanct.Finding, error) {
	return db.GetUnprocessedAWSFindings()
}

func (g GitlabStep) Process(finding *scanct.Finding) ([]scanct.AWSKey, error) {
	client := http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(finding.Repository.GitLab.BaseURL + "/" + finding.Repository.Name + "/-/raw/" + finding.Commit + "/" + finding.File)
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
	log.Info().Str("arn", arn).Msg("found aws key")
	return []scanct.AWSKey{{
		AccessKey: finding.Secret,
		SecretKey: key,
		FindingID: finding.ID,
		Arn:       arn,
	}}, nil
}

func (g GitlabStep) SaveResult(db *scanct.Database, result []scanct.AWSKey) error {
	return db.AddAWSKeys(result)
}

func RunGitlabKeysStep() {
	scanct.RunProcessStep[scanct.Finding, scanct.AWSKey](GitlabStep{}, 5)
}
