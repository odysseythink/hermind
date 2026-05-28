package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaperlessExtension_Name(t *testing.T) {
	p := NewPaperlessExtension()
	assert.Equal(t, "paperless-ngx", p.Name())
}

func TestPaperlessExtension_Handle_UnsupportedMethod(t *testing.T) {
	p := NewPaperlessExtension()
	_, err := p.Handle(context.Background(), "/ext/paperless-ngx", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestPaperlessExtension_Handle_UnknownEndpoint(t *testing.T) {
	p := NewPaperlessExtension()
	_, err := p.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestPaperlessExtension_loadDocuments(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/documents/", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if page == "1" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": 1, "title": "Doc One", "content": "Content one", "created": "2024-01-01"},
				},
				"next": "/api/documents/?page=2",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": 2, "title": "Doc Two", "content": "Content two", "created": "2024-01-02"},
				},
			})
		}
	})

	p := NewPaperlessExtensionWithClient(server.Client())
	body, _ := json.Marshal(PaperlessRequest{BaseURL: server.URL, APIToken: "token"})
	resp, err := p.loadDocuments(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	docs, ok := resp.Data["documents"].([]PaperlessDocument)
	require.True(t, ok)
	assert.Len(t, docs, 2)
	assert.Equal(t, "Doc One", docs[0].Title)
	assert.Equal(t, "Content one", docs[0].Content)
}

func TestPaperlessExtension_loadDocuments_InvalidBody(t *testing.T) {
	p := NewPaperlessExtension()
	_, err := p.loadDocuments(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestPaperlessExtension_loadDocuments_MissingBaseURL(t *testing.T) {
	p := NewPaperlessExtension()
	body, _ := json.Marshal(PaperlessRequest{})
	_, err := p.loadDocuments(context.Background(), body)
	assert.Error(t, err)
}

func TestPaperlessExtension_loadDocuments_APIError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"detail":"Invalid token"}`)
	})

	p := NewPaperlessExtensionWithClient(server.Client())
	body, _ := json.Marshal(PaperlessRequest{BaseURL: server.URL})
	_, err := p.loadDocuments(context.Background(), body)
	assert.Error(t, err)
}
