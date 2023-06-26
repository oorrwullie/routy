package models

const accessLogFilename string = "access.log"

func WriteToAccessLog(data string) error {
	return appendToFile(accessLogFilename, data)
}
