package socketflow

// import (
// 	"bytes"
// 	"crypto/rand"
// 	"encoding/hex"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"log"
// 	"math"
// 	"sync"
// 	"time"

// 	"github.com/gorilla/websocket"
// )

// // Message represents the structure of a WebSocket message
// type Message struct {
// 	ID          string `json:"id"`          // Unique message ID
// 	Topic       string `json:"topic"`       // Topic for the message
// 	Payload     []byte `json:"payload"`     // Payload data
// 	IsChunk     bool   `json:"isChunk"`     // Indicates if this message is a chunk
// 	ChunkIndex  int    `json:"chunkIndex"`  // Index of the chunk
// 	TotalChunks int    `json:"totalChunks"` // Total number of chunks (if IsChunk is true)
// }

// // RetryConfig defines the retry behavior for WebSocket operations
// type RetryConfig struct {
// 	MaxRetries      int           // Maximum number of retries
// 	InitialDelay    time.Duration // Initial delay between retries
// 	MaxDelay        time.Duration // Maximum delay between retries
// 	ExponentialBase float64       // Base for exponential backoff
// }

// // DefaultRetryConfig provides strong defaults for retry behavior
// var DefaultRetryConfig = RetryConfig{
// 	MaxRetries:      5,
// 	InitialDelay:    1 * time.Second,
// 	MaxDelay:        30 * time.Second,
// 	ExponentialBase: 2.0,
// }

// // WebSocketClient represents a WebSocket client
// type WebSocketClient struct {
// 	conn           *websocket.Conn
// 	sendMutex      sync.Mutex
// 	chunkBuffer    map[string][]*Message    // Buffer to store chunks for reassembly
// 	subscriptions  map[string]chan *Message // Map of topics to channels
// 	subscribeMutex sync.Mutex
// 	chunkSize      int
// 	chunkTimeout   time.Duration
// 	retryConfig    RetryConfig // Retry configuration
// }

// // NewWebSocketClient creates a new WebSocket client
// func NewWebSocketClient(conn *websocket.Conn, chunkSize int, chunkTimeout time.Duration) *WebSocketClient {
// 	return &WebSocketClient{
// 		conn:          conn,
// 		chunkBuffer:   make(map[string][]*Message),
// 		subscriptions: make(map[string]chan *Message),
// 		chunkSize:     chunkSize,
// 		chunkTimeout:  chunkTimeout,
// 		retryConfig:   DefaultRetryConfig, // Use default retry config
// 	}
// }

// // SetRetryConfig sets the retry configuration for the client
// func (client *WebSocketClient) SetRetryConfig(config RetryConfig) {
// 	client.retryConfig = config
// }

// // generateMessageID generates a unique message ID
// func generateMessageID() string {
// 	buf := make([]byte, 16)
// 	_, err := rand.Read(buf)
// 	if err != nil {
// 		log.Fatal("Failed to generate message ID:", err)
// 	}
// 	return hex.EncodeToString(buf)
// }

// // SendMessage sends a message to a specific topic, automatically chunking if necessary
// func (client *WebSocketClient) SendMessage(topic string, payload []byte) error {
// 	return client.retry(func() error {
// 		client.sendMutex.Lock()
// 		defer client.sendMutex.Unlock()

// 		// Create a new message with a unique ID
// 		message := &Message{
// 			ID:      generateMessageID(),
// 			Topic:   topic,
// 			Payload: payload,
// 		}

// 		// Automatically chunk the payload if it exceeds the chunk size
// 		if len(message.Payload) > client.chunkSize {
// 			return client.sendChunkedMessage(message)
// 		}

// 		// Send the message as-is if it's small
// 		data, err := json.Marshal(message)
// 		if err != nil {
// 			return fmt.Errorf("failed to marshal message: %w", err)
// 		}

// 		return client.conn.WriteMessage(websocket.TextMessage, data)
// 	})
// }

// // sendChunkedMessage sends a large payload by splitting it into chunks
// func (client *WebSocketClient) sendChunkedMessage(message *Message) error {
// 	totalChunks := (len(message.Payload) + client.chunkSize - 1) / client.chunkSize

// 	for i := 0; i < totalChunks; i++ {
// 		start := i * client.chunkSize
// 		end := start + client.chunkSize
// 		if end > len(message.Payload) {
// 			end = len(message.Payload)
// 		}

// 		chunk := &Message{
// 			ID:          message.ID,
// 			Topic:       message.Topic,
// 			Payload:     message.Payload[start:end],
// 			IsChunk:     true,
// 			ChunkIndex:  i,
// 			TotalChunks: totalChunks,
// 		}

// 		data, err := json.Marshal(chunk)
// 		if err != nil {
// 			return fmt.Errorf("failed to marshal chunk %d: %w", i, err)
// 		}

// 		if err := client.conn.WriteMessage(websocket.TextMessage, data); err != nil {
// 			return fmt.Errorf("failed to send chunk %d: %w", i, err)
// 		}
// 	}

// 	return nil
// }

// // retry implements smart retries with exponential backoff
// func (client *WebSocketClient) retry(operation func() error) error {
// 	var err error
// 	for i := 0; i < client.retryConfig.MaxRetries; i++ {
// 		if err = operation(); err == nil {
// 			return nil
// 		}

// 		// Calculate delay with exponential backoff
// 		delay := time.Duration(math.Pow(client.retryConfig.ExponentialBase, float64(i))) * client.retryConfig.InitialDelay
// 		if delay > client.retryConfig.MaxDelay {
// 			delay = client.retryConfig.MaxDelay
// 		}

// 		log.Printf("Retrying in %v due to error: %v\n", delay, err)
// 		time.Sleep(delay)
// 	}
// 	return fmt.Errorf("max retries exceeded: %w", err)
// }

// // Subscribe subscribes to a topic and returns a channel to receive messages
// func (client *WebSocketClient) Subscribe(topic string) chan *Message {
// 	client.subscribeMutex.Lock()
// 	defer client.subscribeMutex.Unlock()

// 	if ch, exists := client.subscriptions[topic]; exists {
// 		return ch
// 	}

// 	ch := make(chan *Message, 10) // Buffered channel to avoid blocking
// 	client.subscriptions[topic] = ch
// 	return ch
// }

// // Unsubscribe unsubscribes from a topic
// func (client *WebSocketClient) Unsubscribe(topic string) {
// 	client.subscribeMutex.Lock()
// 	defer client.subscribeMutex.Unlock()

// 	if ch, exists := client.subscriptions[topic]; exists {
// 		close(ch)
// 		delete(client.subscriptions, topic)
// 	}
// }

// // ReceiveMessages listens for incoming messages and routes them to the appropriate topic channels
// func (client *WebSocketClient) ReceiveMessages() {
// 	for {
// 		_, data, err := client.conn.ReadMessage()
// 		if err != nil {
// 			log.Printf("Failed to read message: %v\n", err)
// 			break
// 		}

// 		var message Message
// 		if err := json.Unmarshal(data, &message); err != nil {
// 			log.Printf("Failed to unmarshal message: %v\n", err)
// 			continue
// 		}
// 		fmt.Printf("Received message: %s\n", string(data))
// 		if message.IsChunk {
// 			client.handleChunk(message)
// 		} else {
// 			client.routeMessage(message)
// 		}
// 	}
// }

// // handleChunk handles incoming chunks and reassembles the payload
// func (client *WebSocketClient) handleChunk(message Message) {

// 	chunks, exists := client.chunkBuffer[message.ID]
// 	if !exists {
// 		chunks = make([]*Message, message.TotalChunks)
// 		client.chunkBuffer[message.ID] = chunks
// 	}

// 	if message.ChunkIndex < message.TotalChunks {
// 		chunks[message.ChunkIndex] = &message
// 		client.chunkBuffer[message.ID] = chunks

// 		// Check if all chunks are received
// 		allReceived := true
// 		for _, chunk := range chunks {
// 			if chunk == nil {
// 				allReceived = false
// 				break
// 			}
// 		}

// 		if allReceived {
// 			// Reassemble the payload from chunks
// 			reassembledPayload, err := client.reassembleChunks(message.ID)
// 			if err != nil {
// 				log.Printf("Failed to reassemble chunks: %v\n", err)
// 				delete(client.chunkBuffer, message.ID)
// 				return
// 			}

// 			// Create a reassembled message
// 			reassembledMessage := &Message{
// 				ID:      message.ID,
// 				Topic:   message.Topic,
// 				Payload: reassembledPayload,
// 			}

// 			// Route the reassembled message
// 			client.routeMessage(*reassembledMessage)
// 			delete(client.chunkBuffer, message.ID)
// 		}
// 	}
// }

// // routeMessage routes a message to the appropriate topic channel
// func (client *WebSocketClient) routeMessage(message Message) {
// 	client.subscribeMutex.Lock()
// 	defer client.subscribeMutex.Unlock()

// 	if ch, exists := client.subscriptions[message.Topic]; exists {
// 		ch <- &message
// 	} else {
// 		log.Printf("No subscribers for topic: %s\n", message.Topic)
// 	}
// }

// // reassembleChunks reassembles the payload from buffered chunks
// func (client *WebSocketClient) reassembleChunks(messageID string) ([]byte, error) {
// 	chunks, exists := client.chunkBuffer[messageID]
// 	if !exists {
// 		return nil, errors.New("no chunks found for the given message ID")
// 	}

// 	var buffer bytes.Buffer
// 	for _, chunk := range chunks {
// 		if chunk == nil {
// 			return nil, errors.New("missing chunk in sequence")
// 		}
// 		buffer.Write(chunk.Payload)
// 	}

// 	return buffer.Bytes(), nil
// }

// // Close closes the WebSocket connection
// func (client *WebSocketClient) Close() error {
// 	return client.conn.Close()
// }
