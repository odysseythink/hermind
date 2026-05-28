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

func TestDrupalWikiExtension_Name(t *testing.T) {
	d := NewDrupalWikiExtension()
	assert.Equal(t, "drupalwiki", d.Name())
}

func TestDrupalWikiExtension_Handle_UnsupportedMethod(t *testing.T) {
	d := NewDrupalWikiExtension()
	_, err := d.Handle(context.Background(), "/ext/drupalwiki", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestDrupalWikiExtension_Handle_UnknownEndpoint(t *testing.T) {
	d := NewDrupalWikiExtension()
	_, err := d.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestDrupalWikiExtension_loadPages(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/jsonapi/node/wiki_page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id": "1",
					"attributes": map[string]interface{}{
						"title": "Page One",
						"body":  map[string]string{"value": "Content one"},
					},
				},
				{
					"id": "2",
					"attributes": map[string]interface{}{
						"title": "Page Two",
						"body":  map[string]string{"value": "Content two"},
					},
				},
			},
		})
	})

	d := NewDrupalWikiExtensionWithClient(server.Client())
	body, _ := json.Marshal(DrupalWikiRequest{BaseURL: server.URL})
	resp, err := d.loadPages(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	pages, ok := resp.Data["pages"].([]DrupalWikiPage)
	require.True(t, ok)
	assert.Len(t, pages, 2)
	assert.Equal(t, "Page One", pages[0].Title)
	assert.Equal(t, "Content one", pages[0].Content)
}

func TestDrupalWikiExtension_loadPages_InvalidBody(t *testing.T) {
	d := NewDrupalWikiExtension()
	_, err := d.loadPages(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestDrupalWikiExtension_loadPages_MissingBaseURL(t *testing.T) {
	d := NewDrupalWikiExtension()
	body, _ := json.Marshal(DrupalWikiRequest{})
	_, err := d.loadPages(context.Background(), body)
	assert.Error(t, err)
}

func TestDrupalWikiExtension_loadPages_APIError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/jsonapi/node/wiki_page", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"message":"Unauthorized"}`)
	})

	d := NewDrupalWikiExtensionWithClient(server.Client())
	body, _ := json.Marshal(DrupalWikiRequest{BaseURL: server.URL})
	_, err := d.loadPages(context.Background(), body)
	assert.Error(t, err)
}
