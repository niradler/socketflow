package main

import (
	"bytes"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/socketflow"
)

func main() {
	// Connect to the WebSocket server
	var conn *websocket.Conn
	var err error
	for {
		conn, _, err = websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
		if err == nil {
			break
		}
		log.Println("Failed to connect to WebSocket server, retrying in 3 seconds:", err)
		time.Sleep(3 * time.Second)
	}
	defer conn.Close()

	// Create a new WebSocket client
	client := socketflow.NewWebSocketClient(conn, socketflow.Config{
		ChunkSize: 1024,
	})

	conn.EnableWriteCompression(true)
	// Subscribe to status updates
	statusChan := client.SubscribeToStatus()
	go func() {
		for status := range statusChan {
			log.Printf("Received status: %v\n", status)
		}
	}()

	// Start tracking metrics
	client.TrackMetrics(time.Minute * 1)
	client.StartHeartbeat(time.Second * 15)

	// Send a small message
	id, err := client.SendMessage("test-topic", []byte("Hello, World!"))
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}
	log.Printf("Sent small message with ID: %s\n", id)

	// Send a large message
	pattern := []byte("abcd")
	largePayload := bytes.Repeat(pattern, 1500/len(pattern))
	id, err = client.SendMessage("test-topic", []byte(largePayload))
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}
	log.Printf("Sent large message with ID: %s\n", id)

	// Subscribe to a topic
	ch := client.Subscribe("test-topic")
	go func() {
		for msg := range ch {
			log.Printf("Received topic message: ID=%s, Topic=%s, IsChunk=%v, Chunk=%d, TotalChunks=%d, PayloadLen=%v\n", msg.ID, msg.Topic, msg.IsChunk, msg.Chunk, msg.TotalChunks, len(msg.Payload))
			log.Println("Payload:", string(msg.Payload))
		}
	}()

	client.ReceiveMessages()
}
