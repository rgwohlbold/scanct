package main

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	GitLabApiEndpoint string `yaml:"gitlab_api_endpoint"`
	GitLabApiToken    string `yaml:"gitlab_api_token"`
}

type ConfigSignature struct {
	Name     string `yaml:"name"`
	Part     string `yaml:"part"`
	Match    string `yaml:"match,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
	Verifier string `yaml:"verifier,omitempty"`
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
