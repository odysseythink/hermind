package extensions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/scraper"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResyncExtension_Name(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	assert.Equal(t, "resync", r.Name())
}

func TestResyncExtension_Handle_UnsupportedMethod(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	_, err := r.Handle(context.Background(), "/ext/resync-source-document", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestResyncExtension_Handle_UnknownEndpoint(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	_, err := r.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestResyncExtension_resync_InvalidBody(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	_, err := r.resync(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestResyncExtension_resync_MissingType(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	body, _ := json.Marshal(ResyncRequest{})
	_, err := r.resync(context.Background(), body)
	assert.Error(t, err)
}

func TestResyncExtension_resync_UnsupportedType(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	body, _ := json.Marshal(ResyncRequest{Type: "unknown"})
	_, err := r.resync(context.Background(), body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported resync type")
}

func TestResyncExtension_resyncLink(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>Hello World</body></html>"))
	})

	adapter := external.NewChromedpAdapter(nil)
	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)
	manager := scraper.NewManager(adapter, tokenizer)

	r := NewResyncExtension(manager, nil)
	body, _ := json.Marshal(ResyncRequest{
		Type:    "link",
		Options: map[string]interface{}{"url": server.URL},
	})
	resp, err := r.Handle(context.Background(), "/ext/resync-source-document", "POST", body)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Data["content"], "Hello World")
}

func TestResyncExtension_resyncLink_MissingURL(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	body, _ := json.Marshal(ResyncRequest{
		Type:    "link",
		Options: map[string]interface{}{},
	})
	_, err := r.Handle(context.Background(), "/ext/resync-source-document", "POST", body)
	assert.Error(t, err)
}

func TestResyncExtension_resyncLink_NoManager(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	body, _ := json.Marshal(ResyncRequest{
		Type:    "link",
		Options: map[string]interface{}{"url": "http://example.com"},
	})
	_, err := r.Handle(context.Background(), "/ext/resync-source-document", "POST", body)
	assert.Error(t, err)
}

func TestResyncExtension_resyncYouTube_MissingURL(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	body, _ := json.Marshal(ResyncRequest{
		Type:    "youtube",
		Options: map[string]interface{}{},
	})
	_, err := r.Handle(context.Background(), "/ext/resync-source-document", "POST", body)
	assert.Error(t, err)
}

func TestResyncExtension_resyncYouTube(t *testing.T) {
	// This test will fail to extract transcript due to invalid video ID,
	// but verifies the dispatch path works.
	r := NewResyncExtension(nil, map[string]Extension{
		"youtube-transcript": NewYouTubeExtension(),
	})
	body, _ := json.Marshal(ResyncRequest{
		Type:    "youtube",
		Options: map[string]interface{}{"url": "https://www.youtube.com/watch?v=invalid"},
	})
	_, err := r.Handle(context.Background(), "/ext/resync-source-document", "POST", body)
	assert.Error(t, err)
}

func TestResyncExtension_resyncExtension_NotAvailable(t *testing.T) {
	r := NewResyncExtension(nil, nil)
	body, _ := json.Marshal(ResyncRequest{
		Type:    "confluence",
		Options: map[string]interface{}{},
	})
	_, err := r.Handle(context.Background(), "/ext/resync-source-document", "POST", body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestResyncExtension_resyncExtension_Available(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/wiki/api/v2/spaces/TEST/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]string{{"id": "1", "title": "Page"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"body": map[string]interface{}{
				"storage": map[string]string{"value": "<p>Content</p>"},
			},
		})
	})

	c := NewConfluenceExtensionWithClient(server.Client())
	r := NewResyncExtension(nil, map[string]Extension{"confluence": c})
	body, _ := json.Marshal(ResyncRequest{
		Type:    "confluence",
		Options: map[string]interface{}{"baseUrl": server.URL, "spaceKey": "TEST"},
	})
	resp, err := r.Handle(context.Background(), "/ext/resync-source-document", "POST", body)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}
