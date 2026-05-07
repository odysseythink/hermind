package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const bingDefaultURL = "https://www.bing.com/search"

type bingProvider struct {
	client   *http.Client
	endpoint string
	market   string
}

func newBingProvider(market, endpoint string) *bingProvider {
	return &bingProvider{
		client:   &http.Client{Timeout: httpTimeout},
		endpoint: endpoint,
		market:   market,
	}
}

func (p *bingProvider) ID() string       { return "bing" }
func (p *bingProvider) Configured() bool { return true }

func (p *bingProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = bingDefaultURL
	}
	log.Printf("[Bing] Search start: query=%q num_results=%d endpoint=%s market=%q", q, n, endpoint, p.market)

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("bing: invalid endpoint: %w", err)
	}
	qval := u.Query()
	qval.Set("q", q)
	qval.Set("count", fmt.Sprintf("%d", n))
	if p.market != "" {
		qval.Set("setmkt", p.market)
	}
	u.RawQuery = qval.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bing: request error: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:125.0) Gecko/20100101 Firefox/125.0")
	log.Printf("[Bing] Request URL: %s", u.String())

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[Bing] Request failed: %v", err)
		return nil, fmt.Errorf("bing: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[Bing] Response status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bing: http %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bing: read body error: %w", err)
	}
	preview := string(bodyBytes)
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	log.Printf("[Bing] Response body preview: %s", preview)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("bing: parse error: %w", err)
	}

	bodyText := doc.Text()
	if strings.Contains(strings.ToLower(bodyText), "captcha") {
		log.Printf("[Bing] CAPTCHA detected in response body")
		return nil, fmt.Errorf("bing: captcha challenge detected")
	}

	algoCount := doc.Find("li.b_algo").Length()
	log.Printf("[Bing] Found %d li.b_algo elements in HTML", algoCount)

	results := make([]SearchResult, 0, n)
	doc.Find("li.b_algo").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(results) >= n {
			return false
		}
		link := s.Find("h2 a")
		title := strings.TrimSpace(link.Text())
		href, _ := link.Attr("href")
		href = strings.TrimSpace(href)
		snippet := strings.TrimSpace(s.Find(".b_caption p").Text())
		if snippet == "" {
			snippet = strings.TrimSpace(s.Find("p").Text())
		}

		if title != "" && href != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     decodeBingURL(href),
				Snippet: snippet,
			})
		} else {
			log.Printf("[Bing] Skipping result %d: title_empty=%v href_empty=%v", i, title == "", href == "")
		}
		return true
	})

	log.Printf("[Bing] Search complete: returned %d results", len(results))
	return results, nil
}

// decodeBingURL attempts to extract the original destination URL from Bing's
// redirect wrapper links (e.g. https://www.bing.com/ck/a?...&u=<base64>).
// If the href is not a Bing redirect or decoding fails, the original href
// is returned unchanged.
func decodeBingURL(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	if !strings.Contains(u.Host, "bing.com") {
		return href
	}
	encoded := u.Query().Get("u")
	if encoded == "" {
		return href
	}
	decoded, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return href
		}
	}
	result := string(decoded)
	if strings.HasPrefix(result, "http://") || strings.HasPrefix(result, "https://") {
		return result
	}
	return href
}
