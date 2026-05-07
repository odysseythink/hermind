package web

import (
	"context"
	"fmt"
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

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bing: http %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bing: parse error: %w", err)
	}

	bodyText := doc.Text()
	if strings.Contains(strings.ToLower(bodyText), "captcha") {
		return nil, fmt.Errorf("bing: captcha challenge detected")
	}

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
				URL:     href,
				Snippet: snippet,
			})
		}
		return true
	})

	return results, nil
}
