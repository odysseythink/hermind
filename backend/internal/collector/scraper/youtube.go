package scraper

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// YouTubeScraper handles YouTube video transcript extraction.
type YouTubeScraper struct{}

// NewYouTubeScraper creates a new YouTubeScraper.
func NewYouTubeScraper() *YouTubeScraper {
	return &YouTubeScraper{}
}

// ExtractTranscript extracts the transcript text for a YouTube video URL.
func (y *YouTubeScraper) ExtractTranscript(ctx context.Context, videoURL string) (string, error) {
	videoID := extractVideoID(videoURL)
	if videoID == "" {
		return "", fmt.Errorf("could not extract video ID from URL: %s", videoURL)
	}

	watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	playerResponse, err := extractPlayerResponse(string(body))
	if err != nil {
		return "", err
	}

	captionTracks, err := extractCaptionTracks(playerResponse)
	if err != nil {
		return "", err
	}
	if len(captionTracks) == 0 {
		return "", fmt.Errorf("no captions available for video %s", videoID)
	}

	// Prefer English if available.
	track := captionTracks[0]
	for _, t := range captionTracks {
		if strings.Contains(strings.ToLower(t.LanguageCode), "en") {
			track = t
			break
		}
	}

	captionURL := track.BaseURL
	transcript, err := fetchCaptionXML(ctx, captionURL)
	if err != nil {
		return "", err
	}

	return transcript, nil
}

// Scrape extracts the transcript and persists it as a Document.
func (y *YouTubeScraper) Scrape(ctx context.Context, videoURL string, metadata map[string]string, storageDir string, tokenizer *utils.Tokenizer) (*core.ProcessResponse, error) {
	content, err := y.ExtractTranscript(ctx, videoURL)
	if err != nil {
		return nil, err
	}

	title := metadata["title"]
	if title == "" {
		title = videoURL
	}

	doc := &core.Document{
		URL:         videoURL,
		Title:       title,
		DocSource:   "URL link uploaded by the user.",
		ChunkSource: "link://" + videoURL,
		PageContent: content,
	}

	enrichDocument(doc, content, tokenizer)

	filename := utils.SlugifyFilename(title)
	if filename == "" {
		filename = "youtube-link"
	}

	doc, err = utils.WriteToServerDocuments(storageDir, doc, filename, false)
	if err != nil {
		return nil, fmt.Errorf("save document: %w", err)
	}

	return &core.ProcessResponse{
		Filename:  doc.Location,
		Success:   true,
		Reason:    "",
		Documents: []core.Document{*doc},
	}, nil
}

func extractVideoID(videoURL string) string {
	u, err := url.Parse(videoURL)
	if err != nil {
		return ""
	}

	switch {
	case strings.Contains(u.Host, "youtu.be"):
		return strings.TrimPrefix(u.Path, "/")
	case strings.Contains(u.Host, "youtube.com"):
		q := u.Query()
		return q.Get("v")
	}
	return ""
}

var ytInitialPlayerResponseRe = regexp.MustCompile(`ytInitialPlayerResponse\s*=\s*(\{.+?\});`)

func extractPlayerResponse(html string) (map[string]interface{}, error) {
	matches := ytInitialPlayerResponseRe.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("ytInitialPlayerResponse not found")
	}

	jsonStr := matches[1]
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parse player response: %w", err)
	}
	return result, nil
}

type captionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Name         string `json:"name"`
}

func extractCaptionTracks(playerResponse map[string]interface{}) ([]captionTrack, error) {
	captionsRaw, ok := playerResponse["captions"]
	if !ok {
		return nil, nil
	}
	captions, ok := captionsRaw.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	tracklistRaw, ok := captions["playerCaptionsTracklistRenderer"]
	if !ok {
		return nil, nil
	}
	tracklist, ok := tracklistRaw.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	tracksRaw, ok := tracklist["captionTracks"]
	if !ok {
		return nil, nil
	}
	tracksSlice, ok := tracksRaw.([]interface{})
	if !ok {
		return nil, nil
	}

	var tracks []captionTrack
	for _, tr := range tracksSlice {
		trMap, ok := tr.(map[string]interface{})
		if !ok {
			continue
		}
		track := captionTrack{}
		if v, ok := trMap["baseUrl"].(string); ok {
			track.BaseURL = v
		}
		if v, ok := trMap["languageCode"].(string); ok {
			track.LanguageCode = v
		}
		if v, ok := trMap["name"].(map[string]interface{}); ok {
			if simpleText, ok := v["simpleText"].(string); ok {
				track.Name = simpleText
			}
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

// Transcript represents the root element of YouTube's timedtext XML.
type Transcript struct {
	TextNodes []TextNode `xml:"text"`
}

// TextNode represents a single <text> element in the transcript.
type TextNode struct {
	Content string `xml:",chardata"`
}

func fetchCaptionXML(ctx context.Context, captionURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, captionURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	decoder := xml.NewDecoder(resp.Body)
	var transcript Transcript
	if err := decoder.Decode(&transcript); err != nil {
		return "", fmt.Errorf("decode caption XML: %w", err)
	}

	var parts []string
	for _, node := range transcript.TextNodes {
		// Decode HTML entities such as &amp;, &lt;, etc.
		text := html.UnescapeString(node.Content)
		text = strings.TrimSpace(text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no transcript text found in caption XML")
	}
	return strings.Join(parts, " "), nil
}
