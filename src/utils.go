package socketflow

import (
	"crypto/rand"
	"encoding/hex"
	"log"
)

// generateMessageID generates a unique message ID
func generateMessageID() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		log.Fatal("Failed to generate message ID:", err)
	}
	return hex.EncodeToString(buf)
}
