package models

import (
	"encoding/json"
)

const routesFilename string = "routes.json"

type Route struct {
	Subdomain string `json:"subdomain"`
	Target    string `json:"target"`
}

func GetRoutes() ([]Route, error) {
	data := make([]Route, 0)

	res, err := getFileData(routesFilename)
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
