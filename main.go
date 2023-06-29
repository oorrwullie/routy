package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oorrwullie/routy/internal/handlers"
	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
)

func main() {
	eventLog := make(chan logging.EventLogMessage)
	go logging.EventLogger(eventLog)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdownChan

		msg := "Received shutdown signal. Performing graceful shutdown..."
		eventLog <- logging.EventLogMessage{
			Level:   "INFO",
			Caller:  "shutdown()",
			Message: msg,
		}

		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()

	cfg, err := models.GetConfig()
	if err != nil {
		log.Fatal("could not initialize the server")
	}

	r := handlers.NewRouty(
		cfg.Port,
		cfg.Hostname,
		eventLog,
	)

	msg := "Application is running..."
	eventLog <- logging.EventLogMessage{
		Level:   "INFO",
		Caller:  "Main()",
		Message: msg,
	}

	err = r.Route()
	if err != nil {
		eventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "main()->r.Route()",
			Message: err.Error(),
		}
	}
}
