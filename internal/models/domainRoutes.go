package models

import (
	"gopkg.in/yaml.v2"
)

const configFilename string = "cfg.yaml"

type (
	Routes struct {
		Domains []Domain `yaml:"domains"`
	}

	Domain struct {
		Name       string      `yaml:"name"`
		Subdomains []Subdomain `yaml:"subdomains"`
		Paths      []Path      `yaml:"paths"`
	}

	Subdomain struct {
		Name  string `yaml:"name"`
		Paths []Path `yaml:"paths"`
	}

	Path struct {
		Location   string `yaml:"location"`
		Upgrade    bool   `yaml:"upgrade"`
		Target     string `yaml:"target"`
		ListenPort int    `yaml:"listenPort,omitempty"`
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
