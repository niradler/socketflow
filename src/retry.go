package socketflow

import (
	"fmt"
	"log"
	"math"
	"time"
)

// RetryConfig defines the retry behavior for WebSocket operations
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	ExponentialBase float64
}

// DefaultRetryConfig provides strong defaults for retry behavior
var DefaultRetryConfig = RetryConfig{
	MaxRetries:      5,
	InitialDelay:    1 * time.Second,
	MaxDelay:        30 * time.Second,
	ExponentialBase: 2.0,
}

// retry implements smart retries with exponential backoff
func (client *WebSocketClient) retry(operation func() error) error {
	var err error
	for i := 0; i < client.retryConfig.MaxRetries; i++ {
		if err = operation(); err == nil {
			return nil
		}
		delay := time.Duration(math.Pow(client.retryConfig.ExponentialBase, float64(i))) * client.retryConfig.InitialDelay
		if delay > client.retryConfig.MaxDelay {
			delay = client.retryConfig.MaxDelay
		}
		log.Printf("Retrying in %v due to error: %v\n", delay, err)
		time.Sleep(delay)
	}
	return fmt.Errorf("max retries exceeded: %w", err)
}
