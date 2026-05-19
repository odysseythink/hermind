// tool/web/scrape.go
package web

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const scrapePageTimeout = 15 * time.Second

var mdConverter = converter.NewConverter(
	converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
	),
)

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
// Caller must call cleanup() when done.
func newBrowser(ctx context.Context) (*rod.Browser, func() error, error) {
	l := launcher.New()
	if path, found := launcher.LookPath(); found {
		l.Bin(path)
	}
	u, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).Context(ctx)
	if err := browser.Connect(); err != nil {
		l.Cleanup()
		return nil, nil, fmt.Errorf("connect to browser: %w", err)
	}

	cleanup := func() error {
		if err := browser.Close(); err != nil {
			return err
		}
		l.Cleanup()
		return nil
	}
	return browser, cleanup, nil
}

// scrapePage navigates to url, waits for load, and extracts title + body text
// along with all absolute HTTP(S) links on the page.
// format is "text" or "markdown".
// Returns an error if navigation or extraction fails.
// Uses context timeout internally. Closes the page before returning.
func scrapePage(ctx context.Context, browser *rod.Browser, url, format string, waitIdle time.Duration) (*pageContent, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, scrapePageTimeout)
	defer cancel()

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, nil, fmt.Errorf("open page %s: %w", url, err)
	}
	defer page.Close()

	if err := page.Context(ctx).WaitIdle(waitIdle); err != nil {
		return nil, nil, fmt.Errorf("wait idle %s: %w", url, err)
	}

	titleRes, err := page.Context(ctx).Eval("() => document.title")
	if err != nil {
		return nil, nil, fmt.Errorf("extract title %s: %w", url, err)
	}
	title := titleRes.Value.String()

	var content string
	if format == "text" {
		textRes, err := page.Context(ctx).Eval(`() => {
    const el = document.querySelector('article, main, [role="main"]');
    if (el) return el.innerText;
    return document.body ? document.body.innerText : '';
}`)
		if err != nil {
			return nil, nil, fmt.Errorf("extract text %s: %w", url, err)
		}
		content = textRes.Value.String()
	} else {
		html, err := page.Context(ctx).HTML()
		if err != nil {
			return nil, nil, fmt.Errorf("extract html %s: %w", url, err)
		}
		content, err = mdConverter.ConvertString(html)
		if err != nil {
			return nil, nil, fmt.Errorf("convert markdown %s: %w", url, err)
		}
	}

	// Extract links before closing page
	linkRes, err := page.Context(ctx).Eval("() => Array.from(document.querySelectorAll('a[href]')).map(a => a.href)")
	if err != nil {
		return nil, nil, fmt.Errorf("extract links %s: %w", url, err)
	}
	var allLinks []string
	if err := linkRes.Value.Unmarshal(&allLinks); err != nil {
		return nil, nil, fmt.Errorf("unmarshal links %s: %w", url, err)
	}
	var links []string
	for _, link := range allLinks {
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			links = append(links, link)
		}
	}

	return &pageContent{
		URL:     url,
		Title:   title,
		Content: content,
	}, links, nil
}

// extractLinksFromPage returns all absolute HTTP(S) <a href> URLs found on the given page.
// Returns an empty slice (not error) on navigation failure.
// Closes the page before returning.
func extractLinksFromPage(ctx context.Context, browser *rod.Browser, pageURL string, waitIdle time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, scrapePageTimeout)
	defer cancel()

	page, err := browser.Page(proto.TargetCreateTarget{URL: pageURL})
	if err != nil {
		return []string{}, nil // navigation failure → silent skip
	}
	defer page.Close()

	if err := page.Context(ctx).WaitIdle(waitIdle); err != nil {
		return []string{}, nil // load failure → silent skip
	}

	res, err := page.Context(ctx).Eval("() => Array.from(document.querySelectorAll('a[href]')).map(a => a.href)")
	if err != nil {
		return nil, fmt.Errorf("eval links on %s: %w", pageURL, err)
	}

	var allLinks []string
	if err := res.Value.Unmarshal(&allLinks); err != nil {
		return nil, fmt.Errorf("unmarshal links on %s: %w", pageURL, err)
	}

	var links []string
	for _, link := range allLinks {
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			links = append(links, link)
		}
	}
	return links, nil
}
