package external

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChromedpAdapter_FetchText(t *testing.T) {
	adapter := NewChromedpAdapter(nil)
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a simple data URL to avoid network dependencies.
	url := "data:text/html,<html><body><h1>Hello Chromedp</h1></body></html>"
	text, err := adapter.FetchText(ctx, url, nil)

	// chromedp may or may not be available in the test environment.
	if err != nil {
		assert.Contains(t, err.Error(), "chromedp")
		return
	}

	require.NoError(t, err)
	assert.Contains(t, text, "Hello Chromedp")
}

func TestChromedpAdapter_Close_Idempotent(t *testing.T) {
	adapter := NewChromedpAdapter(nil)
	adapter.Close()
	// Second close should not panic.
	adapter.Close()
}
