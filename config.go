package socketflow

import "time"

type Config struct {
	ChunkSize      int
	ChunkTimeout   time.Duration
	RetryConfig    RetryConfig
	MetricInterval time.Duration
}
