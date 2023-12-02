package models

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

const configFilename string = "cfg.yaml"

type (
	Routes struct {
		Domains []Domain `yaml:"domains"`
	}

	Domain struct {
		Name       string      `yaml:"name"`
		Target     string      `yaml:"target,omitempty"`
		Subdomains []Subdomain `yaml:"subdomains"`
	}

	Subdomain struct {
		Name   string `yaml:"name"`
		Target string `yaml:"target"`
		Wss    bool   `yaml:"wss"`
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

	fmt.Printf("%+v\n", data)
	return data, err
}
