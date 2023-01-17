package core

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	GitLabApiEndpoint            string            `yaml:"gitlab_api_endpoint"`
	GitLabApiToken               string            `yaml:"gitlab_api_token"`
	BlacklistedStrings           []string          `yaml:"blacklisted_strings"`
	BlacklistedExtensions        []string          `yaml:"blacklisted_extensions"`
	BlacklistedPaths             []string          `yaml:"blacklisted_paths"`
	BlacklistedEntropyExtensions []string          `yaml:"blacklisted_entropy_extensions"`
	Signatures                   []ConfigSignature `yaml:"signatures"`
}

type ConfigSignature struct {
	Name     string `yaml:"name"`
	Part     string `yaml:"part"`
	Match    string `yaml:"match,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
	Verifier string `yaml:"verifier,omitempty"`
}

func ParseConfig(options *Options) (*Config, error) {
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
