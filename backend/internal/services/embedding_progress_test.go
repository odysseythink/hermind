package services

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEmbeddingProgressManager(t *testing.T) {
	mgr := NewEmbeddingProgressManager()

	w := httptest.NewRecorder()
	connID, done := mgr.AddConnection("test-ws", w)
	assert.NotEmpty(t, connID)

	mgr.BroadcastProgress("test-ws", "doc1.pdf", 50)
	time.Sleep(50 * time.Millisecond)

	body := w.Body.String()
	assert.Contains(t, body, "progress")
	assert.Contains(t, body, "doc1.pdf")
	assert.Contains(t, body, "50")

	mgr.RemoveConnection("test-ws", connID)
	select {
	case <-done:
		// expected
	case <-time.After(time.Second):
		t.Fatal("done channel not closed")
	}
}
