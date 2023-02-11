package jenkins

import (
	"encoding/json"
	"fmt"
	"github.com/rgwohlbold/scanct"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"time"
)

type JobsApiResponse struct {
	Jobs []JobsApiResponseJob `json:"jobs"`
}

type JobsApiResponseJob struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type JobStep struct{}

func (j JobStep) UnprocessedInputs(db *scanct.Database) ([]scanct.Jenkins, error) {
	return db.GetUnprocessedJenkins()
}

func (j JobStep) Process(jenkins *scanct.Jenkins) ([]scanct.JenkinsJob, error) {
	log.Info().Str("jenkins", jenkins.BaseURL).Msg("processing jenkins")
	httpClient := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := httpClient.Get(fmt.Sprintf("%s/api/json", jenkins.BaseURL))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		log.Info().Str("instance", jenkins.BaseURL).Int("status", resp.StatusCode).Msg("unexpected status code")
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var jobs JobsApiResponse
	err = json.Unmarshal(body, &jobs)
	if err != nil {
		return nil, err
	}
	result := make([]scanct.JenkinsJob, len(jobs.Jobs))
	for i, job := range jobs.Jobs {
		result[i] = scanct.JenkinsJob{
			JenkinsID: jenkins.ID,
			Name:      job.Name,
			URL:       fmt.Sprintf("%s/job/%s", jenkins.BaseURL, job.Name),
		}
	}
	return result, nil
}

func (j JobStep) SetProcessed(db *scanct.Database, i *scanct.Jenkins) error {
	return db.SetJenkinsProcessed(i)
}

func (j JobStep) SaveResult(db *scanct.Database, o []scanct.JenkinsJob) error {
	return db.AddJenkinsJob(o)
}

func ImportJobs() {
	scanct.RunProcessStep[scanct.Jenkins, scanct.JenkinsJob](JobStep{}, 5)
}
