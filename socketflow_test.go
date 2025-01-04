package socketflow

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var (
	shortMessageId string
	longMessageId  string
)

func TestWebSocketClient(t *testing.T) {
	// Setup WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade WebSocket connection: %v", err)
		}
		defer conn.Close()

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			// Echo the exact message back
			if err := conn.WriteMessage(messageType, data); err != nil {
				t.Errorf("Failed to send message: %v", err)
				return
			}
		}
	}))
	defer server.Close()

	// Connect to the WebSocket server
	url := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer conn.Close()

	// Create WebSocket client with config
	client := NewWebSocketClient(conn, Config{
		ChunkSize:        1024,
		ReassembleChunks: true,
	})

	// Start receiving messages in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		client.ReceiveMessages()
	}()

	// Ensure the ReceiveMessages goroutine is stopped when the test ends
	defer func() {
		client.Close()
		wg.Wait()
	}()

	// Test SendMessage with small payload
	t.Run("SendMessage with small payload", func(t *testing.T) {
		payload := []byte("Hello, World!")
		id, err := client.SendMessage("test-topic", payload)
		shortMessageId = id
		assert.NoError(t, err, "SendMessage failed with small payload")
	})

	// Test SendMessage with large payload (chunking)
	t.Run("SendMessage with large payload", func(t *testing.T) {
		payload := make([]byte, 1500) // Larger than chunk size
		for i := range payload {
			payload[i] = byte(i % 256) // Fill with non-zero data
		}
		id, err := client.SendMessage("test-topic", payload)
		longMessageId = id
		assert.NoError(t, err, "SendMessage failed with large payload")
	})

	// Test Subscribe and ReceiveMessages
	t.Run("Subscribe and ReceiveMessages", func(t *testing.T) {
		topic := "test-topic"
		ch := client.Subscribe(topic)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case msg := <-ch:
				assert.Equal(t, topic, msg.Topic, "Received message has incorrect topic")
				if msg.ID == shortMessageId {
					assert.Equal(t, "Hello, World!", string(msg.Payload), "Received message has incorrect payload")
				} else if msg.ID == longMessageId {
					assert.Equal(t, 1500, len(msg.Payload), "Received message has incorrect payload length")
				} else {
					t.Errorf("Received message has incorrect ID: %v", msg.ID)
				}
			case <-time.After(5 * time.Second):
				t.Error("Timeout waiting for message")
			}
		}()

		// Simulate sending a message to the server
		msg := Message{Topic: topic, Payload: []byte("Hello, Subscriber!")}
		data, _ := json.Marshal(msg)
		err := conn.WriteMessage(websocket.TextMessage, data)
		assert.NoError(t, err, "Failed to send message to WebSocket server")

		wg.Wait()
	})

	// Test Unsubscribe
	t.Run("Unsubscribe", func(t *testing.T) {
		topic := "test-topic"
		client.Unsubscribe(topic)
		_, exists := client.subscriptions[topic]
		assert.False(t, exists, "Unsubscribe failed to remove topic subscription")
	})

	// Test Status Updates
	t.Run("Status Updates", func(t *testing.T) {
		statusChan := client.SubscribeToStatus()
		go func() {
			for status := range statusChan {
				switch status.Type {
				case "error":
					t.Logf("Error: %v", status.Error)
				case "retry":
					t.Logf("Retrying: %v", status.Message)
				}
			}
		}()

		// Simulate an error
		client.clientStatusChan <- Status{
			Type:    "error",
			Message: "Simulated error",
			Error:   errors.New("test error"),
		}
	})

	// Test Close
	t.Run("Close", func(t *testing.T) {
		err := client.Close()
		assert.NoError(t, err, "Close failed")
	})
}
