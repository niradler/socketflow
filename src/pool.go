package socketflow

import (
	"sync"

	"github.com/gorilla/websocket"
)

type ConnectionPool struct {
	connections []*websocket.Conn
	mu          sync.Mutex
}

func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		connections: make([]*websocket.Conn, 0),
	}
}

func (p *ConnectionPool) Add(conn *websocket.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connections = append(p.connections, conn)
}

func (p *ConnectionPool) Get() *websocket.Conn {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.connections) == 0 {
		return nil
	}
	conn := p.connections[0]
	p.connections = p.connections[1:]
	return conn
}
