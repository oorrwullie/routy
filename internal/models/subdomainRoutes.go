package models

import (
	"encoding/json"
)

const subdomainRoutesFilename string = "subdomainRoutes.json"

type SubdomainRoute struct {
	Domain    string `json:"domain"` // Must be a domain in cfg
	Subdomain string `json:"subdomain"`
	Target    string `json:"target"`
}

func GetDomains() ([]string, error) {
	data, err := GetSubdomainRoutes()
	if err != nil {
		return nil, err
	}

	ds := make([]string, 0)

	for _, d := range data {
		ds = append(ds, d.Domain)
	}

	return ds, nil
}

func GetSubdomainRoutes() ([]SubdomainRoute, error) {
	data := make([]SubdomainRoute, 0)

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

	err = json.Unmarshal(res, &data)
	if err != nil {
		return nil, err
	}

	return data, err
}
