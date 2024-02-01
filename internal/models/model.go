package models

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
)

type Model struct {
	DataDir string
}

func NewModel() (*Model, error) {
	var dataDir string

	if _, err := os.Stat("/var/routy"); err == nil {
		dataDir = "/var/routy"
	} else {
		usr, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("failed to get user's home directory: %s", err)
		}

		dataDir = filepath.Join(usr.HomeDir, "routy")
	}

	return &Model{DataDir: dataDir}, nil
}

func (m *Model) getFileData(filename string) ([]byte, error) {
	fp, err := m.GetFilepath(filename)
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

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v\n", err)
	}

	return data, nil
}

func (m *Model) overwriteFile(filename string, data []byte) error {
	fp, err := m.GetFilepath(filename)
	if err != nil {
		return err
	}

	err = os.WriteFile(fp, data, 0600)
	if err != nil {
		return fmt.Errorf("unable to overwrite file: %v", err)
	}

	return nil
}

func (m *Model) appendToFile(filename string, data string) error {
	fp, err := m.GetFilepath(filename)
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

func (m *Model) GetFilepath(filename string) (string, error) {
	fp := filepath.Join(m.DataDir, filename)

	if _, err := os.Stat(fp); err != nil {
		err = os.MkdirAll(path.Dir(fp), 0750)
		if err != nil {
			return "", fmt.Errorf("unable to create directory: %v", err)
		}
	}

	return fp, nil
}
