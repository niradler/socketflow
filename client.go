package socketflow

import (
	"bytes"
	"encoding/base64"
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

func (client *WebSocketClient) SendMessage(topic string, payload []byte, options *SendOptions) (string, error) {
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

		if options == nil {
			options = &SendOptions{Compress: false}
		}

		var payloadBytes []byte
		if options.Compress {
			compressed, err := compressPayload(payload)
			if err != nil {
				return fmt.Errorf("failed to compress payload: %w", err)
			}
			payloadBytes = compressed
		} else {
			payloadBytes = payload
		}

		payloadStr := base64.StdEncoding.EncodeToString(payloadBytes)

		message := &Message{
			ID:         id,
			Topic:      topic,
			Payload:    payloadStr,
			IsChunk:    false,
			Compressed: options.Compress,
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
			ChunkIndex:  i,
			TotalChunks: totalChunks,
		}

		data, err := client.serializer.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("failed to marshal chunk %d: %w", i, err)
		}

		if err := client.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return fmt.Errorf("failed to send chunk %d: %w", i, err)
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
		_, data, err := client.conn.ReadMessage()
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

		if message.IsChunk {
			client.handleChunk(message)
		} else {
			client.routeMessage(message)
			client.metrics.mu.Lock()
			client.metrics.MessagesReceived++
			client.metrics.mu.Unlock()
		}
	}
}

func (client *WebSocketClient) handleChunk(message Message) {
	chunks, exists := client.chunkBuffer[message.ID]
	if !exists {
		chunks = make([]*Message, message.TotalChunks)
		client.chunkBuffer[message.ID] = chunks
	}

	if message.ChunkIndex < message.TotalChunks {
		chunks[message.ChunkIndex] = &message
		client.chunkBuffer[message.ID] = chunks

		allReceived := true
		for _, chunk := range chunks {
			if chunk == nil {
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
	log.Printf("routeMessage : ID=%s, Topic=%s, Payload=%s\n", message.ID, message.Topic, message.Payload)
	payloadBytes, err := base64.StdEncoding.DecodeString(message.Payload)
	if err != nil {
		client.metrics.mu.Lock()
		client.metrics.Errors++
		client.metrics.mu.Unlock()

		client.clientStatusChan <- Status{
			Type:    "error",
			Message: "Failed to decode Base64 payload",
			Error:   err,
		}
		return
	}

	log.Printf("routeMessage before: ID=%s, Topic=%s, Payload=%s\n", message.ID, message.Topic, payloadBytes)
	if message.Compressed {
		decompressed, err := decompressPayload(payloadBytes)
		if err != nil {
			client.metrics.mu.Lock()
			client.metrics.Errors++
			client.metrics.mu.Unlock()

			client.clientStatusChan <- Status{
				Type:    "error",
				Message: "Failed to decompress payload",
				Error:   err,
			}
			return
		}
		payloadBytes = decompressed
	}
	log.Printf("routeMessage after: ID=%s, Topic=%s, Payload=%s\n", message.ID, message.Topic, payloadBytes)
	if ch, exists := client.subscriptions[message.Topic]; exists {
		ch <- &Message{
			ID:      message.ID,
			Topic:   message.Topic,
			Payload: string(payloadBytes),
		}
	} else {
		log.Printf("No subscribers for topic: %s\n", message.Topic)
	}
}

func (client *WebSocketClient) reassembleChunks(messageID string) (string, error) {
	chunks, exists := client.chunkBuffer[messageID]
	if !exists {
		return "", errors.New("no chunks found for the given message ID")
	}

	var buffer bytes.Buffer
	for _, chunk := range chunks {
		if chunk == nil {
			return "", errors.New("missing chunk in sequence")
		}
		buffer.WriteString(chunk.Payload)
	}

	return buffer.String(), nil
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
					return
				}
			}
		}
	}()
}
