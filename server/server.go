package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/socketflow"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("Failed to upgrade WebSocket connection:", err)
	}
	defer conn.Close()

	client := socketflow.NewWebSocketClient(conn, socketflow.Config{
		ChunkSize:    1024,
		ChunkTimeout: 5 * time.Second,
	})

	ch := client.Subscribe("test-topic")
	go func() {
		for msg := range ch {
			log.Printf("Received message: ID=%s, Topic=%s, Payload=%s\n", msg.ID, msg.Topic, msg.Payload)
		}
	}()

	// Start receiving messages
	client.ReceiveMessages()
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	log.Println("WebSocket server started at :8080")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
