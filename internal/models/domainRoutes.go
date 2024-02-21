package models

import (
	"gopkg.in/yaml.v2"
)

const configFilename string = "cfg.yaml"

type (
	Routes struct {
		Http Http `yaml:"http"`
		Ssh  Ssh  `yaml:"ssh"`
	}

	Http struct {
		Domains []Domain `yaml:"domains"`
	}

	Domain struct {
		Name       string      `yaml:"name"`
		Paths      []Path      `yaml:"paths"`
		Subdomains []Subdomain `yaml:"subdomains"`
	}

	Subdomain struct {
		Name  string `yaml:"name"`
		Paths []Path `yaml:"paths"`
	}

	Path struct {
		ListenPort int    `yaml:"listenPort,omitempty"`
		Location   string `yaml:"location"`
		Target     string `yaml:"target"`
		Upgrade    bool   `yaml:"upgrade"`
	}

	Ssh struct {
		Configs    []SshConfig
		Enabled    bool
		ListenPort int
	}

	SshConfig struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}
)

func GetDomainRoutes() (*Routes, error) {
	data := &Routes{}

	m, err := NewModel()
	if err != nil {
		return nil, err
	}

	res, err := m.getFileData(configFilename)
	if err != nil {
		if err.Error() == "file not found" {
			return data, nil
		} else {
			return nil, err
		}
	}

	err = yaml.Unmarshal(res, data)
	if err != nil {
		return nil, err
	}

	return data, err
}
