package models

const eventLogFilename string = "events.log"

func WriteToEventLog(data string) error {
	return appendToFile(eventLogFilename, data)
}
