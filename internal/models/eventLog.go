package models

const eventLogFilename string = "events.log"

func (m *Model) WriteToEventLog(data string) error {
	return m.appendToFile(eventLogFilename, data)
}
