package models

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

const dataDir string = "routy"

func getFileData(filename string) ([]byte, error) {
	fp, err := GetFilepath(filename)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(fp); err != nil {
		return nil, fmt.Errorf("file not found")
	}

	file, err := os.Open(fp)
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
	fp, err := GetFilepath(filename)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fp, data, 0600)
	if err != nil {
		return fmt.Errorf("unable to overwrite file: %v", err)
	}

	return nil
}

func appendToFile(filename string, data string) error {
	fp, err := GetFilepath(filename)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
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

func GetFilepath(filename string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}

	fp := filepath.Join(home, dataDir, filename)

	if _, err := os.Stat(fp); err != nil {
		err = os.MkdirAll(path.Dir(fp), 0750)
		if err != nil {
			return "", fmt.Errorf("unable to create directory: %v", err)
		}
	}

	return fp, nil
}
