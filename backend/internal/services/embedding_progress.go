package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type EmbedProgressEvent struct {
	Type     string `json:"type"` // "progress", "complete", "error"
	Message  string `json:"message"`
	Percent  int    `json:"percent,omitempty"`
	Document string `json:"document,omitempty"`
}

type sseConn struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
	ch      chan []byte
}

// run is the per-connection send goroutine. It serialises all writes to the
// ResponseWriter so concurrent Broadcast calls cannot race on the writer.
func (c *sseConn) run() {
	for {
		select {
		case data := <-c.ch:
			c.writer.Write(data)
			c.flusher.Flush()
		case <-c.done:
			return
		}
	}
}

type EmbeddingProgressManager struct {
	mu    sync.RWMutex
	conns map[string]map[string]*sseConn // workspaceSlug -> connID -> conn
}

func NewEmbeddingProgressManager() *EmbeddingProgressManager {
	return &EmbeddingProgressManager{
		conns: make(map[string]map[string]*sseConn),
	}
}

func (m *EmbeddingProgressManager) AddConnection(workspaceSlug string, w http.ResponseWriter) (connID string, done chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	connID = fmt.Sprintf("%d", time.Now().UnixNano())
	done = make(chan struct{})

	if m.conns[workspaceSlug] == nil {
		m.conns[workspaceSlug] = make(map[string]*sseConn)
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		close(done)
		return connID, done
	}
	conn := &sseConn{writer: w, flusher: flusher, done: done, ch: make(chan []byte, 16)}
	m.conns[workspaceSlug][connID] = conn
	go conn.run()
	return connID, done
}

func (m *EmbeddingProgressManager) RemoveConnection(workspaceSlug, connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conns, ok := m.conns[workspaceSlug]; ok {
		if conn, ok := conns[connID]; ok {
			close(conn.done)
			delete(conns, connID)
		}
		if len(conns) == 0 {
			delete(m.conns, workspaceSlug)
		}
	}
}

func (m *EmbeddingProgressManager) Broadcast(workspaceSlug string, event EmbedProgressEvent) {
	m.mu.RLock()
	conns := m.conns[workspaceSlug]
	m.mu.RUnlock()

	if len(conns) == 0 {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	payload := fmt.Sprintf("data: %s\n\n", data)

	for id, conn := range conns {
		select {
		case conn.ch <- []byte(payload):
		default:
			// Channel full -> client is too slow; close and clean up.
			m.mu.Lock()
			close(conn.done)
			delete(m.conns[workspaceSlug], id)
			if len(m.conns[workspaceSlug]) == 0 {
				delete(m.conns, workspaceSlug)
			}
			m.mu.Unlock()
		}
	}
}

func (m *EmbeddingProgressManager) BroadcastProgress(workspaceSlug, document string, percent int) {
	m.Broadcast(workspaceSlug, EmbedProgressEvent{
		Type:     "progress",
		Message:  fmt.Sprintf("Embedding %s", document),
		Percent:  percent,
		Document: document,
	})
}
