package models

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const dataDir string = "routy"

func getFileData(filename string) ([]byte, error) {
	home, err := os.UserHomeDir()
	filePath := path.Join(home, dataDir, filename)

	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file not found")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v\n", err)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v\n", err)
	}

	return data, nil
}

func overwriteFile(filename string, data []byte) error {
	home, err := os.UserHomeDir()
	filePath := path.Join(home, dataDir, filename)

	err = os.MkdirAll(path.Dir(filePath), 0750)
	if err != nil {
		return fmt.Errorf("unable to create directory: %v", err)
	}

	err = ioutil.WriteFile(filePath, data, 0600)
	if err != nil {
		return fmt.Errorf("unable to overwrite file: %v", err)
	}

	return nil
}

func appendToFile(filename string, data string) error {
	home, err := os.UserHomeDir()
	filePath := path.Join(home, dataDir, filename)

	err = os.MkdirAll(path.Dir(filePath), 0750)
	if err != nil {
		return fmt.Errorf("unable to create directory: %v", err)
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(data)
	if err != nil {
		return fmt.Errorf("failed to write to file: %v", err)
	}

	return nil
}
