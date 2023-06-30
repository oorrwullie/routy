package logging

import (
	"fmt"
	"time"

	"github.com/oorrwullie/routy/internal/models"
)

type EventLogMessage struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Caller    string `json:"caller"`
	Message   string `json:"message"`
}

func StartEventLogger(logChan <-chan EventLogMessage) error {
	m, err := models.NewModel()
	if err != nil {
		return err
	}

	for logMsg := range logChan {
		t := time.Now()
		logMsg.Timestamp = t.Format("15:04:05 MST 10-02-2006")

		entry := fmt.Sprintf(
			`{"Timestamp": "%s", "Level": "%s", "Caller": "%s", "Message": "%s"}
`,
			logMsg.Timestamp,
			logMsg.Level,
			logMsg.Caller,
			logMsg.Message,
		)

		m.WriteToEventLog(entry)
	}

	return nil
}
