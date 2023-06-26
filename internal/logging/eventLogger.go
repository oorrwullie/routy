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

func EventLogger(logChan <-chan EventLogMessage) {
	for logMsg := range logChan {
		t := time.Now()
		logMsg.Timestamp = t.Format("15:04:05 MST 10-02-2006")

		entry := fmt.Sprintf(
			`{"Timestamp": "%s", "Level": "%s", "Caller": "%s", "Message": "%s"}\n`,
			logMsg.Timestamp,
			logMsg.Level,
			logMsg.Caller,
			logMsg.Message,
		)

		models.WriteToEventLog(entry)
	}
}
