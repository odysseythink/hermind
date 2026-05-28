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

func TestConfluenceExtension_Name(t *testing.T) {
	c := NewConfluenceExtension()
	assert.Equal(t, "confluence", c.Name())
}

func TestConfluenceExtension_Handle_UnsupportedMethod(t *testing.T) {
	c := NewConfluenceExtension()
	_, err := c.Handle(context.Background(), "/ext/confluence", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestConfluenceExtension_Handle_UnknownEndpoint(t *testing.T) {
	c := NewConfluenceExtension()
	_, err := c.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestConfluenceExtension_loadSpace(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/wiki/api/v2/spaces/TEST/pages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]string{
				{"id": "1", "title": "Page One"},
				{"id": "2", "title": "Page Two"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"body": map[string]interface{}{
				"storage": map[string]string{"value": "<p>Content one</p>"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"body": map[string]interface{}{
				"storage": map[string]string{"value": "<p>Content two</p>"},
			},
		})
	})

	c := NewConfluenceExtensionWithClient(server.Client())
	body, _ := json.Marshal(ConfluenceRequest{BaseURL: server.URL, SpaceKey: "TEST", AccessToken: "token"})
	resp, err := c.loadSpace(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	pages, ok := resp.Data["pages"].([]ConfluencePage)
	require.True(t, ok)
	assert.Len(t, pages, 2)
	assert.Equal(t, "Page One", pages[0].Title)
	assert.Equal(t, "Content one", pages[0].Content)
}

func TestConfluenceExtension_loadSpace_Pagination(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	callCount := 0
	mux.HandleFunc("/wiki/api/v2/spaces/TEST/pages", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		start := r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/json")
		if start == "0" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]string{{"id": "1", "title": "Page One"}},
				"_links":  map[string]string{"next": "/wiki/api/v2/spaces/TEST/pages?start=1"},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]string{{"id": "2", "title": "Page Two"}},
			})
		}
	})
	mux.HandleFunc("/wiki/api/v2/pages/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"body": map[string]interface{}{"storage": map[string]string{"value": "<p>One</p>"}}})
	})
	mux.HandleFunc("/wiki/api/v2/pages/2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"body": map[string]interface{}{"storage": map[string]string{"value": "<p>Two</p>"}}})
	})

	c := NewConfluenceExtensionWithClient(server.Client())
	body, _ := json.Marshal(ConfluenceRequest{BaseURL: server.URL, SpaceKey: "TEST"})
	resp, err := c.loadSpace(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 2, callCount)

	pages, _ := resp.Data["pages"].([]ConfluencePage)
	assert.Len(t, pages, 2)
}

func TestConfluenceExtension_loadSpace_InvalidBody(t *testing.T) {
	c := NewConfluenceExtension()
	_, err := c.loadSpace(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestConfluenceExtension_loadSpace_MissingFields(t *testing.T) {
	c := NewConfluenceExtension()
	body, _ := json.Marshal(ConfluenceRequest{})
	_, err := c.loadSpace(context.Background(), body)
	assert.Error(t, err)
}

func TestConfluenceExtension_loadSpace_APIError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/wiki/api/v2/spaces/TEST/pages", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"message":"Unauthorized"}`)
	})

	c := NewConfluenceExtensionWithClient(server.Client())
	body, _ := json.Marshal(ConfluenceRequest{BaseURL: server.URL, SpaceKey: "TEST"})
	_, err := c.loadSpace(context.Background(), body)
	assert.Error(t, err)
}

func TestBasicAuth(t *testing.T) {
	encoded := basicAuth("user", "pass")
	assert.Equal(t, "dXNlcjpwYXNz", encoded)
}

func TestStripHTMLTags(t *testing.T) {
	assert.Equal(t, "Hello World", stripHTMLTags("<p>Hello World</p>"))
	assert.Equal(t, "a b", stripHTMLTags("<div>a <span>b</span></div>"))
}
