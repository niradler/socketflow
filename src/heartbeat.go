package socketflow

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// StartHeartbeat sends periodic ping messages to keep the connection alive
func (client *WebSocketClient) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("Failed to send heartbeat: %v\n", err)
					return
				}
			}
		}
	}()
}
