package socketflow

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	ID          string `json:"id"`
	Topic       string `json:"topic"`
	Payload     []byte `json:"payload"`
	IsChunk     bool   `json:"isChunk"`
	Chunk       int    `json:"chunkIndex"`
	TotalChunks int    `json:"totalChunks"`
}

type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	ExponentialBase float64
}

var DefaultRetryConfig = RetryConfig{
	MaxRetries:      5,
	InitialDelay:    1 * time.Second,
	MaxDelay:        30 * time.Second,
	ExponentialBase: 2.0,
}

type Status struct {
	Type    string
	Message string
	Error   error
}

type Metrics struct {
	MessagesSent     int
	MessagesReceived int
	Latency          time.Duration
	Errors           int
	mu               sync.Mutex
}

func NewWebSocketClient(conn *websocket.Conn, config Config) *WebSocketClient {
	if config.RetryConfig.MaxRetries == 0 {
		config.RetryConfig = DefaultRetryConfig
	}

	return &WebSocketClient{
		conn:             conn,
		chunkBuffer:      make(map[string][]*Message),
		subscriptions:    make(map[string]chan *Message),
		config:           config,
		serializer:       JSONSerializer{},
		metrics:          &Metrics{},
		clientStatusChan: make(chan Status, 10),
	}
}
