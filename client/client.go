package main

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/socketflow"
)

func main() {
	// Connect to the WebSocket server
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		log.Fatal("Failed to connect to WebSocket server:", err)
	}
	defer conn.Close()

	// Create a new WebSocket client
	client := socketflow.NewWebSocketClient(conn, socketflow.Config{
		ChunkSize:    1024,
		ChunkTimeout: 5 * time.Second,
	})

	// Subscribe to status updates
	statusChan := client.SubscribeToStatus()
	go func() {
		for status := range statusChan {
			switch status.Type {
			case "error":
				log.Printf("Error: %v\n", status.Error)
			case "retry":
				log.Printf("Retrying: %v\n", status.Message)
			}
		}
	}()

	// Start tracking metrics
	// client.TrackMetrics()

	// Send a small message without compression
	id, err := client.SendMessage("test-topic", []byte("Hello, World!"), nil)
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}
	log.Printf("Sent small message with ID: %s\n", id)

	// Send a large message with compression
	largePayload := string(make([]byte, 1500)) // Larger than chunk size
	id, err = client.SendMessage("test-topic", []byte(largePayload), &socketflow.SendOptions{Compress: true})
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}
	log.Printf("Sent large compressed message with ID: %s\n", id)

	// Subscribe to a topic
	ch := client.Subscribe("test-topic")
	go func() {
		for msg := range ch {
			log.Printf("Received message: ID=%s, Topic=%s, Payload=%s\n", msg.ID, msg.Topic, msg.Payload)
		}
	}()

	client.ReceiveMessages()
}
