package gitlab

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rgwohlbold/scanct"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
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

const DoRegister = false
const RegisterEmail = "scanct-testing2@rgwohlbold.de"
const RegisterUsername = "scanct-testing2"

func Signup(httpClient *http.Client, gl *scanct.GitLab) error {
	r := regexp.MustCompile("name=\"authenticity_token\" value=\"([^\"]+)\"")
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	httpClient.Jar = jar
	resp, err := httpClient.Get(fmt.Sprintf("%s/users/sign_up", gl.BaseURL))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("bad status code %d", resp.StatusCode))
	}
	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	bodyStr := string(body)
	submatches := r.FindStringSubmatch(bodyStr)
	if len(submatches) != 2 {
		return errors.New("could not find authenticity token")
	}
	authToken := submatches[1]

	b := make([]byte, 20)
	_, err = rand.Read(b)
	if err != nil {
		return err
	}
	password := hex.EncodeToString(b)
	values := url.Values{}
	values.Add("new_user[first_name]", RegisterUsername)
	values.Add("new_user[last_name]", RegisterUsername)
	values.Add("new_user[username]", RegisterUsername)
	values.Add("new_user[email]", RegisterEmail)
	values.Add("new_user[password]", password)
	values.Add("authenticity_token", authToken)

	resp, err = httpClient.Post(fmt.Sprintf("%s/users", gl.BaseURL), "application/x-www-form-urlencoded", strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("bad status code %d", resp.StatusCode))
	} else if strings.Contains(string(body), "prohibited this user from being saved") {
		return errors.New("could not create user")
	} else if strings.Contains(string(body), "However, we could not sign you in because your account is awaiting approval from your GitLab administrator.") {
		return errors.New("awaiting approval from admin")
	}
	client, err := gitlab.NewBasicAuthClient(RegisterUsername, password, gitlab.WithBaseURL(gl.URL()))
	if err != nil {
		return err
	}
	currentUser, _, err := client.Users.CurrentUser(nil)
	if err != nil {
		return err
	}
	fmt.Println(currentUser.Name)
	gl.Email = RegisterEmail
	gl.Password = password
	return nil
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
			gl := scanct.GitLab{
				InstanceID:  instance.ID,
				AllowSignup: strings.Contains(bodyStr, RegisterMagicString),
				Email:       "",
				Password:    "",
				APIToken:    "",
				Processed:   false,
				BaseURL:     fmt.Sprintf("https://%s", instance.Name),
			}
			if gl.AllowSignup && DoRegister {
				err = Signup(&client, &gl)
			}
			return []scanct.GitLab{gl}, nil
		}
	}
	// no instance found
	return nil, nil
}

func (g FilterStep) SaveResult(db *scanct.Database, result []scanct.GitLab) error {
	return db.AddGitLab(result)
}

func FilterInstances() {
	scanct.RunProcessStep[scanct.Instance, scanct.GitLab](FilterStep{}, 50)
}
