package main

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	GitLabApiEndpoint string `yaml:"gitlab_api_endpoint"`
	GitLabApiToken    string `yaml:"gitlab_api_token"`
	CertificateLogURI string `yaml:"certificate_log_uri"`
}

func ParseConfig() (*Config, error) {
	config := &Config{}
	var (
		data []byte
		err  error
	)
	data, err = os.ReadFile("config.yaml")
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return config, err
	}
	return config, nil
}
