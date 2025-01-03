package socketflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	conn             *websocket.Conn
	sendMutex        sync.Mutex
	chunkBuffer      map[string][]*Message
	subscriptions    map[string]chan *Message
	subscribeMutex   sync.Mutex
	serializer       Serializer
	metrics          *Metrics
	clientStatusChan chan Status
	config           Config
}

type Serializer interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type JSONSerializer struct{}

func (s JSONSerializer) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (s JSONSerializer) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (client *WebSocketClient) SetRetryConfig(config RetryConfig) {
	client.config.RetryConfig = config
}

func (client *WebSocketClient) SetSerializer(serializer Serializer) {
	client.serializer = serializer
}

func (client *WebSocketClient) SendMessage(topic string, payload []byte) (string, error) {
	id := generateMessageID()
	start := time.Now()
	defer func() {
		client.metrics.mu.Lock()
		client.metrics.MessagesSent++
		client.metrics.Latency = time.Since(start)
		client.metrics.mu.Unlock()
	}()

	return id, client.retry(func() error {
		client.sendMutex.Lock()
		defer client.sendMutex.Unlock()

		message := &Message{
			ID:      id,
			Topic:   topic,
			Payload: payload,
		}

		if len(message.Payload) > client.config.ChunkSize {
			return client.sendChunkedMessage(message)
		}

		data, err := client.serializer.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		return client.conn.WriteMessage(websocket.TextMessage, data)
	})
}

func (client *WebSocketClient) sendChunkedMessage(message *Message) error {
	totalChunks := (len(message.Payload) + client.config.ChunkSize - 1) / client.config.ChunkSize

	for i := 0; i < totalChunks; i++ {
		start := i * client.config.ChunkSize
		end := start + client.config.ChunkSize
		if end > len(message.Payload) {
			end = len(message.Payload)
		}

		chunk := &Message{
			ID:          message.ID,
			Topic:       message.Topic,
			Payload:     message.Payload[start:end],
			IsChunk:     true,
			Chunk:       i + 1, // Start chunk index at 1
			TotalChunks: totalChunks,
		}

		data, err := client.serializer.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("failed to marshal chunk %d: %w", i+1, err)
		}

		if err := client.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			return fmt.Errorf("failed to send chunk %d: %w", i+1, err)
		}
	}

	return nil
}

func (client *WebSocketClient) retry(operation func() error) error {
	var err error
	for i := 0; i < client.config.RetryConfig.MaxRetries; i++ {
		if err = operation(); err == nil {
			return nil
		}

		delay := time.Duration(math.Pow(client.config.RetryConfig.ExponentialBase, float64(i))) * client.config.RetryConfig.InitialDelay
		if delay > client.config.RetryConfig.MaxDelay {
			delay = client.config.RetryConfig.MaxDelay
		}

		client.clientStatusChan <- Status{
			Type:    "retry",
			Message: fmt.Sprintf("Retrying in %v", delay),
			Error:   err,
		}

		time.Sleep(delay)
	}

	client.clientStatusChan <- Status{
		Type:    "error",
		Message: "Max retries exceeded",
		Error:   err,
	}

	return fmt.Errorf("max retries exceeded: %w", err)
}

func (client *WebSocketClient) Subscribe(topic string) chan *Message {
	client.subscribeMutex.Lock()
	defer client.subscribeMutex.Unlock()

	if ch, exists := client.subscriptions[topic]; exists {
		return ch
	}

	ch := make(chan *Message, 10)
	client.subscriptions[topic] = ch
	return ch
}

func (client *WebSocketClient) Unsubscribe(topic string) {
	client.subscribeMutex.Lock()
	defer client.subscribeMutex.Unlock()

	if ch, exists := client.subscriptions[topic]; exists {
		close(ch)
		delete(client.subscriptions, topic)
	}
}

func (client *WebSocketClient) ReceiveMessages() {
	for {
		messageType, data, err := client.conn.ReadMessage()
		if err != nil {
			client.metrics.mu.Lock()
			client.metrics.Errors++
			client.metrics.mu.Unlock()

			client.clientStatusChan <- Status{
				Type:    "error",
				Message: "Failed to read message",
				Error:   err,
			}
			break
		}

		switch messageType {
		case websocket.TextMessage:
			var message Message
			if err := client.serializer.Unmarshal(data, &message); err != nil {
				client.metrics.mu.Lock()
				client.metrics.Errors++
				client.metrics.mu.Unlock()

				client.clientStatusChan <- Status{
					Type:    "error",
					Message: "Failed to unmarshal message",
					Error:   err,
				}
				continue
			}

			client.routeMessage(message)
			client.metrics.mu.Lock()
			client.metrics.MessagesReceived++
			client.metrics.mu.Unlock()

		case websocket.BinaryMessage:
			var message Message
			if err := client.serializer.Unmarshal(data, &message); err != nil {
				client.metrics.mu.Lock()
				client.metrics.Errors++
				client.metrics.mu.Unlock()

				client.clientStatusChan <- Status{
					Type:    "error",
					Message: "Failed to unmarshal binary message",
					Error:   err,
				}
				continue
			}

			if message.IsChunk {
				client.handleChunk(message)
			} else {
				client.routeMessage(message)
				client.metrics.mu.Lock()
				client.metrics.MessagesReceived++
				client.metrics.mu.Unlock()
			}

		case websocket.CloseMessage:
			client.clientStatusChan <- Status{
				Type:    "close",
				Message: "Connection closed",
				Error:   errors.New("connection closed"),
			}
			break

		case websocket.PingMessage:
			log.Println("Received ping message")
			client.conn.WriteMessage(websocket.PongMessage, nil)

		case websocket.PongMessage:
			log.Println("Received pong message")

		default:
			client.metrics.mu.Lock()
			client.metrics.Errors++
			client.metrics.mu.Unlock()

			client.clientStatusChan <- Status{
				Type:    "error",
				Message: "Unsupported message type",
				Error:   fmt.Errorf("unsupported message type: %d", messageType),
			}
		}
	}
}

func (client *WebSocketClient) handleChunk(message Message) {
	chunks, exists := client.chunkBuffer[message.ID]
	if !exists {
		chunks = make([]*Message, message.TotalChunks)
		client.chunkBuffer[message.ID] = chunks
	}

	if message.Chunk <= message.TotalChunks {
		chunks[message.Chunk-1] = &message // Adjust for 1-based indexing
		client.chunkBuffer[message.ID] = chunks

		allReceived := true
		for _, c := range chunks {
			if c == nil {
				allReceived = false
				break
			}
		}

		if allReceived {
			reassembledPayload, err := client.reassembleChunks(message.ID)
			if err != nil {
				client.metrics.mu.Lock()
				client.metrics.Errors++
				client.metrics.mu.Unlock()
				delete(client.chunkBuffer, message.ID)
				return
			}

			reassembledMessage := &Message{
				ID:      message.ID,
				Topic:   message.Topic,
				Payload: reassembledPayload,
			}

			client.routeMessage(*reassembledMessage)
			delete(client.chunkBuffer, message.ID)
		}
	}
}

func (client *WebSocketClient) routeMessage(message Message) {
	client.subscribeMutex.Lock()
	defer client.subscribeMutex.Unlock()

	if ch, exists := client.subscriptions[message.Topic]; exists {
		ch <- &Message{
			ID:      message.ID,
			Topic:   message.Topic,
			Payload: message.Payload,
		}
	} else {
		log.Printf("No subscribers for topic: %s\n", message.Topic)
	}
}

func (client *WebSocketClient) reassembleChunks(messageID string) ([]byte, error) {
	chunks, exists := client.chunkBuffer[messageID]
	if !exists {
		return nil, errors.New("no chunks found for the given message ID")
	}

	var buffer bytes.Buffer
	for _, chunk := range chunks {
		if chunk == nil {
			return nil, errors.New("missing chunk in sequence")
		}
		buffer.Write(chunk.Payload)
	}

	return buffer.Bytes(), nil
}

func (client *WebSocketClient) Close() error {
	return client.conn.Close()
}

func (client *WebSocketClient) SubscribeToStatus() <-chan Status {
	return client.clientStatusChan
}

func (client *WebSocketClient) TrackMetrics(interval time.Duration) {
	go func() {
		for {
			select {
			case <-time.After(interval):
				client.metrics.mu.Lock()
				log.Printf("Metrics: Sent=%d, Received=%d, Latency=%v, Errors=%d\n",
					client.metrics.MessagesSent, client.metrics.MessagesReceived, client.metrics.Latency, client.metrics.Errors)
				client.metrics.mu.Unlock()
			}
		}
	}()
}

func (client *WebSocketClient) StartHeartbeat(interval time.Duration) {
	if interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					client.clientStatusChan <- Status{
						Type:    "error",
						Message: "Failed to send ping message",
						Error:   err,
					}
					return
				}
			}
		}
	}()
}
