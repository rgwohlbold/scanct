package jenkins

import (
	"fmt"
	"github.com/rgwohlbold/scanct"
	"github.com/rs/zerolog/log"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
	"io"
	"net/http"
	"os"
	"time"
)

type JenkinsSecretsFinder struct{}

func (f JenkinsSecretsFinder) UnprocessedInstances(db *scanct.Database) ([]scanct.JenkinsJob, error) {
	return db.GetUnprocessedJenkinsJobs()
}

func (_ JenkinsSecretsFinder) ProcessInstance(job *scanct.JenkinsJob) ([]scanct.JenkinsFinding, error) {
	log.Info().Str("job", job.URL).Msg("processing job")
	httpClient := http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	url := fmt.Sprintf("%s/ws/*zip*/%s.zip", job.URL, job.Name)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		//log.Info().Int("status", resp.StatusCode).Str("url", url).Msg("unexpected status code")
		return nil, nil
	}
	zipPath := fmt.Sprintf("/tmp/%s.zip", scanct.Hash(job.Name))
	var f *os.File
	f, err = os.Create(zipPath)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		scanct.PanicIfError(f.Close())
		return nil, err
	}
	scanct.PanicIfError(f.Close())
	defer func() { scanct.PanicIfError(os.Remove(zipPath)) }()

	dirPath := fmt.Sprintf("/tmp/%s", scanct.Hash(job.Name))

	err = scanct.ExtractZip(zipPath, dirPath)
	defer func() { scanct.PanicIfError(os.RemoveAll(dirPath)) }()

	if err != nil {
		return nil, err
	}

	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, err
	}
	var findings []report.Finding
	findings, err = detector.DetectFiles(dirPath)
	if err != nil {
		return nil, err
	}

	var secrets []scanct.JenkinsFinding
	for _, finding := range findings {
		secret := finding.Secret
		if len(secret) > 50 {
			secret = secret[:50] + "..."
		}
		secrets = append(secrets, scanct.JenkinsFinding{
			JobID:     job.ID,
			Secret:    secret,
			StartLine: finding.StartLine,
			EndLine:   finding.EndLine,
			File:      finding.File,
			URL:       fmt.Sprintf("%s/ws/*zip*/%s.zip", job.URL, job.Name),
			Rule:      finding.RuleID,
		})
	}
	return secrets, nil
}

func (_ JenkinsSecretsFinder) SaveResult(db *scanct.Database, findings []scanct.JenkinsFinding) error {
	return db.SaveJenkinsFindings(findings)
}

func (_ JenkinsSecretsFinder) SetProcessed(db *scanct.Database, job *scanct.JenkinsJob) error {
	return db.SetJenkinsJobProcessed(job)
}

func RunJenkinsSecretsFinder() {
	scanct.RunFilter[scanct.JenkinsJob, scanct.JenkinsFinding](JenkinsSecretsFinder{}, 5)
}
