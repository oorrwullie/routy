package models

import (
	"encoding/json"
	"fmt"
)

const subdomainRoutesFilename string = "cfg.json"

type Route struct {
	Domains map[string][]SubdomainRoute `json:"domains"`
}

type SubdomainRoute struct {
	Subdomain string `json:"subdomain"`
	Target    string `json:"target"`
}

func GetDomains() ([]string, error) {
	data, err := GetSubdomainRoutes()
	if err != nil {
		return nil, err
	}

	ds := make([]string, 0)

	for d, sd := range data.Domains {
		for _, s := range sd {
			ds = append(ds, fmt.Sprintf("%s.%s", s.Subdomain, d))
		}
	}

	return ds, nil
}

func GetSubdomainRoutes() (*Route, error) {
	var data *Route = &Route{}

	m, err := NewModel()
	if err != nil {
		return nil, err
	}

	res, err := m.getFileData(subdomainRoutesFilename)
	if err != nil {
		if err.Error() == "file not found" {
			return data, nil
		} else {
			return nil, err
		}
	}

	err = json.Unmarshal(res, data)
	if err != nil {
		return nil, err
	}

	return data, err
}
