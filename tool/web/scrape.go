// tool/web/scrape.go
package web

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const scrapePageTimeout = 15 * time.Second

// pageContent holds extracted data from a single page.
type pageContent struct {
	URL     string
	Title   string
	Content string
}

// newBrowser launches a headless Chromium via rod.
// It tries to find an existing Chrome/Chromium installation first.
// If not found, rod auto-downloads on first Launch().
// Returns an error (never panics) if launch fails.
func newBrowser() (*rod.Browser, error) {
	l := launcher.New()
	if path, found := launcher.LookPath(); found {
		l.Bin(path)
	}
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect to browser: %w", err)
	}
	return browser, nil
}

// scrapePage navigates to url, waits for load, and extracts title + body text.
// format is "text" or "markdown".
// Returns an error if navigation or extraction fails.
// Uses context timeout internally. Closes the page before returning.
func scrapePage(browser *rod.Browser, url, format string) (*pageContent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), scrapePageTimeout)
	defer cancel()

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open page %s: %w", url, err)
	}
	defer page.Close()

	err = page.Context(ctx).WaitLoad()
	if err != nil {
		return nil, fmt.Errorf("wait load %s: %w", url, err)
	}

	titleRes, err := page.Context(ctx).Eval("() => document.title")
	if err != nil {
		return nil, fmt.Errorf("extract title %s: %w", url, err)
	}
	title := titleRes.Value.String()

	var content string
	if format == "text" {
		textRes, err := page.Context(ctx).Eval("() => document.body.innerText")
		if err != nil {
			return nil, fmt.Errorf("extract text %s: %w", url, err)
		}
		content = textRes.Value.String()
	} else {
		html, err := page.Context(ctx).HTML()
		if err != nil {
			return nil, fmt.Errorf("extract html %s: %w", url, err)
		}
		conv := converter.NewConverter()
		content, err = conv.ConvertString(html)
		if err != nil {
			return nil, fmt.Errorf("convert markdown %s: %w", url, err)
		}
	}

	return &pageContent{
		URL:     url,
		Title:   title,
		Content: content,
	}, nil
}

// extractLinksFromPage returns all absolute <a href> URLs found on the given page
// that share the same origin as baseOrigin.
// Returns an empty slice (not error) on navigation failure.
// Closes the page before returning.
func extractLinksFromPage(browser *rod.Browser, pageURL string, baseOrigin string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), scrapePageTimeout)
	defer cancel()

	page, err := browser.Page(proto.TargetCreateTarget{URL: pageURL})
	if err != nil {
		return []string{}, nil
	}
	defer page.Close()

	err = page.Context(ctx).WaitLoad()
	if err != nil {
		return []string{}, nil
	}

	res, err := page.Context(ctx).Eval("() => Array.from(document.querySelectorAll('a[href]')).map(a => a.href)")
	if err != nil {
		return []string{}, nil
	}

	var allLinks []string
	if err := res.Value.Unmarshal(&allLinks); err != nil {
		return []string{}, nil
	}

	var links []string
	for _, link := range allLinks {
		if link == baseOrigin || strings.HasPrefix(link, baseOrigin+"/") {
			links = append(links, link)
		}
	}
	return links, nil
}
