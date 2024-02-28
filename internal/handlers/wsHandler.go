package handlers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
)

func (r *Routy) handleWebSocket(path models.Path) {
	http.HandleFunc(path.Location, r.wsHandleFunc(path))

	go func(path models.Path) {
		http.ListenAndServe(fmt.Sprintf(":%d", path.ListenPort), nil)
	}(path)
}

// wsHandleFunc handles WebSocket connections
func (r *Routy) wsHandleFunc(path models.Path) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if r.denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
			return
		}

		r.accessLog <- req

		var upgrader = websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}

		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			msg := fmt.Sprintf("Error upgrading connection to WebSocket: %v", err)
			r.EventLog <- logging.EventLogMessage{
				Level:   "ERROR",
				Caller:  "handleWebSocket()->upgrader.Upgrade()",
				Message: msg,
			}

			return
		}
		defer conn.Close()

		targetWs, _, err := websocket.DefaultDialer.Dial(path.Target, req.Header)
		if err != nil {
			msg := fmt.Sprintf("Error connecting to target server: %v", err)
			r.EventLog <- logging.EventLogMessage{
				Level:   "ERROR",
				Caller:  "handleWebSocket()->websocket.DefaultDialer.Dial()",
				Message: msg,
			}

			return
		}
		defer targetWs.Close()

		// Bidirectional proxy
		go func() {
			defer targetWs.Close()
			defer conn.Close()

			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					msg := fmt.Sprintf("Error receiving message from client: %v", err)
					r.EventLog <- logging.EventLogMessage{
						Level:   "ERROR",
						Caller:  "handleWebSocket()->conn.ReadMessage()",
						Message: msg,
					}

					return
				}

				err = targetWs.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					msg := fmt.Sprintf("Error sending message to target server: %v", err)
					r.EventLog <- logging.EventLogMessage{
						Level:   "ERROR",
						Caller:  "handleWebSocket()->targetWs.WriteMessage()",
						Message: msg,
					}

					return
				}
			}
		}()

		for {
			_, message, err := targetWs.ReadMessage()
			if err != nil {
				msg := fmt.Sprintf("Error receiving message from target server: %v", err)
				r.EventLog <- logging.EventLogMessage{
					Level:   "ERROR",
					Caller:  "handleWebSocket()->targetWs.ReadMessage()",
					Message: msg,
				}

				return
			}

			err = conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				msg := fmt.Sprintf("Error sending message to client: %v", err)
				r.EventLog <- logging.EventLogMessage{
					Level:   "ERROR",
					Caller:  "handleWebSocket()->conn.WriteMessage()",
					Message: msg,
				}

				return
			}
		}

	}
}
