package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildFakeYouTubePage(videoID string) string {
	playerResponse := fmt.Sprintf(`{
		"videoDetails": {"videoId": "%s"},
		"captions": {
			"playerCaptionsTracklistRenderer": {
				"captionTracks": [
					{
						"baseUrl": "/captions/%s",
						"languageCode": "en",
						"name": {"simpleText": "English"}
					},
					{
						"baseUrl": "/captions/%s?lang=es",
						"languageCode": "es",
						"name": {"simpleText": "Spanish"}
					}
				]
			}
		}
	}`, videoID, videoID, videoID)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Fake Video</title></head>
<body>
<script>
var ytInitialPlayerResponse = %s;
</script>
</body>
</html>`, playerResponse)
}

func buildFakeCaptionXML() string {
	return `<?xml version="1.0" encoding="utf-8" ?>
<transcript>
<text start="0" dur="2">Hello world</text>
<text start="2" dur="3">This is a test transcript</text>
<text start="5" dur="2">Goodbye</text>
</transcript>`
}

func buildFakeCaptionXML_WithEntitiesAndNestedTags() string {
	return `<?xml version="1.0" encoding="utf-8" ?>
<transcript>
<text start="0" dur="2">Hello &amp; welcome</text>
<text start="2" dur="3">This is a &lt;test&gt; transcript</text>
<text start="5" dur="2">Line one<br/>Line two</text>
<text start="7" dur="1">Goodbye</text>
</transcript>`
}

func TestYouTubeScraper_ExtractTranscript(t *testing.T) {
	videoID := "abc123"
	captionXML := buildFakeCaptionXML()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(buildFakeYouTubePage(videoID)))
		case fmt.Sprintf("/captions/%s", videoID):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(captionXML))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Since extractVideoID expects youtube.com or youtu.be, we test
	// the internal helper functions directly with the mock server.
	ctx := context.Background()
	transcript, err := fetchCaptionXML(ctx, server.URL+"/captions/"+videoID)
	require.NoError(t, err)
	assert.Contains(t, transcript, "Hello world")
	assert.Contains(t, transcript, "This is a test transcript")
	assert.Contains(t, transcript, "Goodbye")
}

func TestYouTubeScraper_ExtractTranscript_NoCaptions(t *testing.T) {
	page := `<!DOCTYPE html>
<html><body>
<script>
var ytInitialPlayerResponse = {"videoDetails": {"videoId": "xyz"}};
</script>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(page))
	}))
	defer server.Close()

	// Can't easily route youtube.com to our test server, so test internal helpers.
	playerResponse, err := extractPlayerResponse(page)
	require.NoError(t, err)
	tracks, err := extractCaptionTracks(playerResponse)
	require.NoError(t, err)
	assert.Len(t, tracks, 0)
}

func TestYouTubeScraper_Scrape(t *testing.T) {
	videoID := "ted123"
	captionXML := buildFakeCaptionXML()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(buildFakeYouTubePage(videoID)))
		case fmt.Sprintf("/captions/%s", videoID):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(captionXML))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// We can't route a real youtube.com URL to our test server without
	// DNS/proxy tricks, so we test the scrape method with a mock that
	// bypasses the HTTP fetch. Instead, we test the save path by
	// manually constructing the scenario.
	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	storageDir := t.TempDir()
	// Directly test the scrape path by providing transcript content.
	ctx := context.Background()
	transcript, err := fetchCaptionXML(ctx, server.URL+"/captions/"+videoID)
	require.NoError(t, err)

	doc := &core.Document{
		URL:         "https://youtu.be/" + videoID,
		Title:       "Test Video",
		DocSource:   "URL link uploaded by the user.",
		ChunkSource: "link://https://youtu.be/" + videoID,
		PageContent: transcript,
	}
	enrichDocument(doc, transcript, tokenizer)

	doc, err = utils.WriteToServerDocuments(storageDir, doc, "test-video", false)
	require.NoError(t, err)
	assert.NotEmpty(t, doc.Location)

	dest := filepath.Join(storageDir, "documents", "custom-documents", "test-video.json")
	_, err = os.Stat(dest)
	require.NoError(t, err)
}

func TestExtractVideoID(t *testing.T) {
	assert.Equal(t, "dQw4w9WgXcQ", extractVideoID("https://www.youtube.com/watch?v=dQw4w9WgXcQ"))
	assert.Equal(t, "dQw4w9WgXcQ", extractVideoID("https://youtube.com/watch?v=dQw4w9WgXcQ"))
	assert.Equal(t, "dQw4w9WgXcQ", extractVideoID("https://youtu.be/dQw4w9WgXcQ"))
	assert.Equal(t, "", extractVideoID("https://example.com"))
	assert.Equal(t, "", extractVideoID("not-a-url"))
}

func TestExtractPlayerResponse(t *testing.T) {
	page := `<script>var ytInitialPlayerResponse = {"key": "value"};</script>`
	resp, err := extractPlayerResponse(page)
	require.NoError(t, err)
	assert.Equal(t, "value", resp["key"])
}

func TestExtractPlayerResponse_Missing(t *testing.T) {
	page := `<html><body>No script here</body></html>`
	_, err := extractPlayerResponse(page)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ytInitialPlayerResponse not found")
}

func TestExtractCaptionTracks(t *testing.T) {
	playerResponse := map[string]interface{}{
		"captions": map[string]interface{}{
			"playerCaptionsTracklistRenderer": map[string]interface{}{
				"captionTracks": []interface{}{
					map[string]interface{}{
						"baseUrl":      "https://example.com/captions/en",
						"languageCode": "en",
						"name": map[string]interface{}{
							"simpleText": "English",
						},
					},
				},
			},
		},
	}
	tracks, err := extractCaptionTracks(playerResponse)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "en", tracks[0].LanguageCode)
	assert.Equal(t, "https://example.com/captions/en", tracks[0].BaseURL)
	assert.Equal(t, "English", tracks[0].Name)
}

func TestExtractCaptionTracks_NoCaptions(t *testing.T) {
	playerResponse := map[string]interface{}{
		"videoDetails": map[string]interface{}{"videoId": "123"},
	}
	tracks, err := extractCaptionTracks(playerResponse)
	require.NoError(t, err)
	assert.Len(t, tracks, 0)
}

func TestFetchCaptionXML_HTMLEntitiesAndNestedTags(t *testing.T) {
	captionXML := buildFakeCaptionXML_WithEntitiesAndNestedTags()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(captionXML))
	}))
	defer server.Close()

	ctx := context.Background()
	transcript, err := fetchCaptionXML(ctx, server.URL)
	require.NoError(t, err)
	assert.Contains(t, transcript, "Hello & welcome")
	assert.Contains(t, transcript, "This is a <test> transcript")
	assert.Contains(t, transcript, "Line one")
	assert.Contains(t, transcript, "Line two")
	assert.Contains(t, transcript, "Goodbye")
}
