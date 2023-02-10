package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
	"github.com/rgwohlbold/scanct"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func ParseSecret(accessKeyId string, body string) (string, string, error) {
	r, err := regexp.Compile("([a-zA-Z0-9/+]{40})")
	if err != nil {
		return "", "", err
	}
	matches := r.FindAllString(body, 100)
	if len(matches) == 100 {
		return "", "", errors.New("too many matches")
	}
	matches = scanct.Unique(matches)
	client := http.Client{Timeout: 5 * time.Second}
	for _, match := range matches {
		if !strings.Contains(match, "EXAMPLE") {
			s := sts.New(session.Must(session.NewSession(&aws.Config{
				Credentials: credentials.NewStaticCredentials(accessKeyId, match, ""),
				HTTPClient:  &client,
			})))
			var stsResp *sts.GetCallerIdentityOutput
			stsResp, err = s.GetCallerIdentity(&sts.GetCallerIdentityInput{})
			if err != nil {
				continue
			}
			return match, *stsResp.Arn, nil
		}
	}
	return "", "", errors.New("no tokens found")
}
