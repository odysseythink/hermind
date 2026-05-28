package external

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// ChromedpAdapter wraps chromedp for browser-based page fetching.
type ChromedpAdapter struct {
	launchArgs  []string
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// NewChromedpAdapter creates a new ChromedpAdapter and allocates a shared browser.
func NewChromedpAdapter(launchArgs []string) *ChromedpAdapter {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("ignore-certificate-errors", true),
	)
	for _, arg := range launchArgs {
		opts = append(opts, chromedp.Flag(arg, true))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	return &ChromedpAdapter{
		launchArgs:  launchArgs,
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
	}
}

// FetchText navigates to the given URL and extracts the page text.
func (c *ChromedpAdapter) FetchText(ctx context.Context, url string, headers map[string]string) (string, error) {
	taskCtx, cancel := chromedp.NewContext(c.allocCtx)
	defer cancel()

	// Respect the caller's deadline but cap at a safe maximum.
	deadline := time.Now().Add(utils.DefaultTimeoutChromedp)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	runCtx, cancel := context.WithDeadline(taskCtx, deadline)
	defer cancel()

	var text string
	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Evaluate("document.body.innerText", &text),
	}

	if len(headers) > 0 {
		// chromedp does not have a simple way to set arbitrary headers per-request
		// without using Network.setExtraHTTPHeaders; for MVP we skip header injection.
		_ = headers
	}

	if err := chromedp.Run(runCtx, actions...); err != nil {
		return "", fmt.Errorf("chromedp fetch failed: %w", err)
	}

	return text, nil
}

// Close shuts down the shared browser allocator.
func (c *ChromedpAdapter) Close() {
	if c.allocCancel != nil {
		c.allocCancel()
	}
}
