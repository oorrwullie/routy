package models

const accessLogFilename string = "access.log"

func (m *Model) WriteToAccessLog(data string) error {
	return m.appendToFile(accessLogFilename, data)
}
