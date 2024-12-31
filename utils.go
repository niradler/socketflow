package socketflow

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
)

func generateMessageID() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		log.Fatal("Failed to generate message ID:", err)
	}
	return hex.EncodeToString(buf)
}

func compressPayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err := gz.Write(payload); err != nil {
		return nil, fmt.Errorf("failed to write payload to gzip writer: %w", err)
	}

	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func decompressPayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	log.Println("decompressPayload : payload=", payload)
	gz, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}
	log.Println("decompressPayload : decompressed=", decompressed)
	log.Printf("decompressPayload : decompressed string=%s", string(decompressed))
	return decompressed, nil
}
