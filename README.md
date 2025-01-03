# SocketFlow

[![Go Reference](https://pkg.go.dev/badge/github.com/niradler/socketflow.svg)](https://pkg.go.dev/github.com/niradler/socketflow)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/niradler/socketflow/blob/main/LICENSE)

SocketFlow is a lightweight, efficient, and easy-to-use WebSocket library for Go. It supports **topic-based messaging**, **automatic chunking for large payloads**, and **real-time two-way communication**. Designed with simplicity and performance in mind, SocketFlow is perfect for building real-time applications like chat systems, live updates, and more.

## Features

- **Topic-Based Messaging**: Send and receive messages based on topics.
- **Automatic Chunking**: Automatically splits large payloads into chunks and reassembles them on the receiving end.
- **Real-Time Two-Way Communication**: Supports both request-response and pub/sub patterns.
- **Thread-Safe**: Uses mutexes to ensure thread safety.
- **Easy-to-Use API**: Simple and intuitive API for sending, receiving, and subscribing to messages.

## Installation

```bash
go get github.com/niradler/socketflow
```

## Usage

### Server

Create a WebSocket server that handles incoming connections and messages:

```go
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
        ChunkSize: 1024,
    })

    ch := client.Subscribe("test-topic")
    go func() {
        for msg := range ch {
            log.Printf("Received topic message: ID=%s, Topic=%s, PayloadLen=%v\n", msg.ID, msg.Topic, len(msg.Payload))
            log.Println("Payload:", string(msg.Payload))
        }
    }()

    client.ReceiveMessages()
}

func main() {
    http.HandleFunc("/ws", handleWebSocket)
    log.Println("WebSocket server started at :8080")
    log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
```

### Client

Create a WebSocket client that connects to the server and sends/receives messages:

```go
package main

import (
    "bytes"
    "log"
    "time"

    "github.com/gorilla/websocket"
    "github.com/niradler/socketflow"
)

func main() {
    conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
    if err != nil {
        log.Fatal("Failed to connect to WebSocket server:", err)
    }
    defer conn.Close()

    client := socketflow.NewWebSocketClient(conn, socketflow.Config{
        ChunkSize: 1024,
    })

    conn.EnableWriteCompression(true)
    statusChan := client.SubscribeToStatus()
    go func() {
        for status := range statusChan {
            log.Printf("Received status: %v\n", status)
        }
    }()

    client.TrackMetrics(time.Minute * 1)
    client.StartHeartbeat(time.Second * 15)

    id, err := client.SendMessage("test-topic", []byte("Hello, World!"))
    if err != nil {
        log.Fatal("Failed to send message:", err)
    }
    log.Printf("Sent small message with ID: %s\n", id)

    pattern := []byte("abcd")
    largePayload := bytes.Repeat(pattern, 1500/len(pattern))
    id, err = client.SendMessage("test-topic", []byte(largePayload))
    if err != nil {
        log.Fatal("Failed to send message:", err)
    }
    log.Printf("Sent large message with ID: %s\n", id)

    ch := client.Subscribe("test-topic")
    go func() {
        for msg := range ch {
            log.Printf("Received message: ID=%s, Topic=%s, Payload=%s\n", msg.ID, msg.Topic, msg.Payload)
        }
    }()

    client.ReceiveMessages()
}
```