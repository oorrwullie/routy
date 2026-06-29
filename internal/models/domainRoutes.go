package models

import (
	"gopkg.in/yaml.v2"
)

const configFilename string = "cfg.yaml"

type (
	Routes struct {
		Domains   []Domain   `yaml:"domains"`
		TLSRoutes []TLSRoute `yaml:"tlsRoutes,omitempty"`
	}

	Domain struct {
		Name       string      `yaml:"name"`
		Subdomains []Subdomain `yaml:"subdomains"`
		Paths      []Path      `yaml:"paths"`
	}

	Subdomain struct {
		Name  string      `yaml:"name"`
		CORS  *CORSConfig `yaml:"cors,omitempty"`
		Paths []Path      `yaml:"paths"`
	}

	CORSConfig struct {
		AllowOrigins     []string `yaml:"allowOrigins,omitempty"`
		AllowMethods     []string `yaml:"allowMethods,omitempty"`
		AllowHeaders     []string `yaml:"allowHeaders,omitempty"`
		ExposeHeaders    []string `yaml:"exposeHeaders,omitempty"`
		AllowCredentials bool     `yaml:"allowCredentials,omitempty"`
		MaxAge           int      `yaml:"maxAge,omitempty"`
	}

	Path struct {
		Location   string `yaml:"location"`
		Upgrade    bool   `yaml:"upgrade"`
		Target     string `yaml:"target"`
		ListenPort int    `yaml:"listenPort,omitempty"`
	}

	// TLSRoute is a raw TLS passthrough route. Routy reads the SNI from the
	// ClientHello and proxies the encrypted TCP stream to Target without
	// terminating TLS.
	TLSRoute struct {
		Host   string `yaml:"host"`
		Target string `yaml:"target"`
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
