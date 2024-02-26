package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oorrwullie/routy/internal/handlers"
	"github.com/oorrwullie/routy/internal/logging"
)

func main() {
	r, err := handlers.NewRouty()
	if err != nil {
		log.Fatalf("Error creating new Routy: %s", err)
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdownChan

		msg := "Received shutdown signal. Performing graceful shutdown..."
		r.EventLog <- logging.EventLogMessage{
			Level:   "INFO",
			Caller:  "shutdown()",
			Message: msg,
		}

		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()

	msg := "Application is running..."
	r.EventLog <- logging.EventLogMessage{
		Level:   "INFO",
		Caller:  "Main()",
		Message: msg,
	}

	err = r.Route()
	if err != nil {
		r.EventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "main()->r.Route()",
			Message: err.Error(),
		}
	}
}
