package socketflow

type Message struct {
	ID          string `json:"id"`
	Topic       string `json:"topic"`
	Payload     []byte `json:"payload"`
	IsChunk     bool   `json:"isChunk"`
	ChunkIndex  int    `json:"chunkIndex"`
	TotalChunks int    `json:"totalChunks"`
	Ack         bool   `json:"ack"`
}
