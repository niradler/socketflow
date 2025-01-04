package socketflow

type Config struct {
	ChunkSize        int
	RetryConfig      RetryConfig
	ReassembleChunks bool
}
