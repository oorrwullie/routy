package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

const configFilename string = "cfg.json"

type Config struct {
	Port     string `json:"port"`
	Hostname string `json:"hostname"`
}

func GetConfig() (*Config, error) {
	var config Config

	m, err := NewModel()
	if err != nil {
		return nil, err
	}

	filePath := path.Join(m.DataDir, configFilename)

	if _, err := os.Stat(filePath); err != nil {
		fmt.Println("It looks like this is the first run. Generating config files...")

		fmt.Println("Enter the domain name of the server: ")
		fmt.Scanln(&config.Hostname)
		fmt.Println("Enter the port the server will listen on: ")
		fmt.Scanln(&config.Port)

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
