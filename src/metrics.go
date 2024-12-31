package socketflow

import (
	"log"
	"time"
)

type Metrics struct {
	MessagesSent     int
	MessagesReceived int
	Latency          time.Duration
	Errors           int
}

func (client *WebSocketClient) TrackMetrics() *Metrics {
	metrics := &Metrics{}
	go func() {
		for {
			select {
			case <-time.After(1 * time.Second):
				log.Printf("Metrics: Sent=%d, Received=%d, Latency=%v, Errors=%d\n",
					metrics.MessagesSent, metrics.MessagesReceived, metrics.Latency, metrics.Errors)
			}
		}
	}()
	return metrics
}
