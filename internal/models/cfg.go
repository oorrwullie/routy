package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

const configFilename string = "cfg.json"

type Config struct {
	Hostnames []string `json:"hostnames"`
}

func GetConfig() (*Config, error) {
	var config Config

	m, err := NewModel()
	if err != nil {
		return nil, err
	}

	filePath := path.Join(m.DataDir, configFilename)

	if _, err := os.Stat(filePath); err != nil {
		var hostname = ""
		fmt.Println("It looks like this is the first run. Generating config files...")

		fmt.Println("Enter the domain name of the server: ")
		fmt.Scanln(&hostname)
		config.Hostnames = append(config.Hostnames, hostname)

		cb, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			return nil, err
		}

		err = m.overwriteFile(configFilename, cb)
		if err != nil {
			return nil, err
		}

		mt := []string{}
		mb, err := json.MarshalIndent(mt, "", "    ")
		if err != nil {
			return nil, err
		}

		err = m.overwriteFile(denyListFilename, mb)
		if err != nil {
			return nil, err
		}

		err = m.overwriteFile(subdomainRoutesFilename, mb)
		if err != nil {
			return nil, err
		}

		return &config, nil
	}

	res, err := m.getFileData(configFilename)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(res, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
