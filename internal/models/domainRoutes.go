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
		Target     string      `yaml:"target,omitempty"`
		Subdomains []Subdomain `yaml:"subdomains"`
	}

	Subdomain struct {
		Name   string      `yaml:"name"`
		Target string      `yaml:"target"`
		Paths  []WebSocket `yaml:"paths,omitempty"`
	}

	WebSocket struct {
		Location    string `yaml:"location"`
		Upgrade     bool   `yaml:"upgrade"`
		Port        int    `yaml:"port"`
		TargetPort  int    `yaml:"target-port"`
		Timeout     int    `yaml:"timeout,omitempty"`
		IdleTimeout int    `yaml:"idle-timeout,omitempty"`
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
