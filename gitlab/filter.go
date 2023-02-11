package gitlab

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rgwohlbold/scanct"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strings"
	"time"
)

const SignInURL = "/users/sign_in"
const SignInMagicString = "<meta content=\"GitLab\" property=\"og:site_name\">"
const RegisterMagicString = "<a data-qa-selector=\"register_link\" href=\"/users/sign_up\">Register now</a>"

type FilterStep struct{}

func (g FilterStep) SetProcessed(db *scanct.Database, i *scanct.Instance) error {
	return db.SetInstanceProcessed(i)
}

func (g FilterStep) UnprocessedInputs(db *scanct.Database) ([]scanct.Instance, error) {
	return db.GetUnprocessedInstancesForGitlab()
}

func (g FilterStep) Process(instance *scanct.Instance) ([]scanct.GitLab, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(fmt.Sprintf("https://%s%s", instance.Name, SignInURL))
	if err != nil {
		if strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
			return nil, nil
		} else if strings.Contains(err.Error(), "tls: failed to verify certificate: x509:") {
			return nil, nil
		} else if strings.Contains(err.Error(), "no such host") {
			return nil, nil
		} else if strings.Contains(err.Error(), "stopped after 10 redirects") {
			return nil, nil

		}
		return nil, errors.Wrap(err, "error requesting instance")
	} else if resp.StatusCode != 200 {
		return nil, nil
	} else {
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		bodyStr := string(body)
		if strings.Contains(bodyStr, SignInMagicString) {
			log.Info().Str("instance", instance.Name).Msg("found gitlab instance")
			return []scanct.GitLab{{
				InstanceID:  instance.ID,
				AllowSignup: strings.Contains(bodyStr, RegisterMagicString),
				Email:       "",
				Password:    "",
				APIToken:    "",
				Processed:   false,
				BaseURL:     fmt.Sprintf("https://%s", instance.Name),
			}}, nil
		}
	}
	// no instance found
	return nil, nil
}

func (g FilterStep) SaveResult(db *scanct.Database, result []scanct.GitLab) error {
	for _, r := range result {
		err := db.AddGitLab(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func FilterInstances() {
	scanct.RunProcessStep[scanct.Instance, scanct.GitLab](FilterStep{}, 50)
}
