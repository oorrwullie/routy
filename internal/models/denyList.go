package models

import "encoding/json"

const denyListFilename string = "denyList.json"

type DenyList struct {
	list []string
}

func GetDenyList() (*DenyList, error) {
	data := make([]string, 0)

	res, err := getFileData(denyListFilename)
	if err != nil {
		if err.Error() == "file not found" {
			return &DenyList{list: data}, nil
		} else {
			return nil, err
		}
	}

	err = json.Unmarshal(res, &data)
	if err != nil {
		return nil, err
	}

	return &DenyList{list: data}, nil
}

func (d *DenyList) IsDenied(ip string) bool {
	for _, v := range d.list {
		if v == ip {
			return true
		}
	}

	return false
}
