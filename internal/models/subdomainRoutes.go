package models

import (
	"encoding/json"
)

const subdomainRoutesFilename string = "subdomainRoutes.json"

type SubdomainRoute struct {
	Subdomain string `json:"subdomain"`
	Target    string `json:"target"`
}

func GetSubdomainRoutes() ([]SubdomainRoute, error) {
	data := make([]SubdomainRoute, 0)

	res, err := getFileData(subdomainRoutesFilename)
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