package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/niradler/socketflow"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:       func(r *http.Request) bool { return true },
	EnableCompression: true,
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("Failed to upgrade WebSocket connection:", err)
	}
	defer conn.Close()

	conn.EnableWriteCompression(true)
	client := socketflow.NewWebSocketClient(conn, socketflow.Config{
		ChunkSize:        1024,
		ReassembleChunks: false,
	})

	ch := client.Subscribe("test-topic")
	go func() {
		for msg := range ch {
			log.Printf("Received topic message: ID=%s, Topic=%s, IsChunk=%v, Chunk=%d, TotalChunks=%d, PayloadLen=%v\n", msg.ID, msg.Topic, msg.IsChunk, msg.Chunk, msg.TotalChunks, len(msg.Payload))
			log.Println("Payload:", string(msg.Payload))
			client.SendMessage("test-topic", []byte("Message received: "+msg.ID))
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
