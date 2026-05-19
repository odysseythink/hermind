// tool/web/robots.go
package web

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

const robotsUserAgent = "HermindBot"
const robotsCacheTTL = 5 * time.Minute

type robotsEntry struct {
	data      *robotstxt.RobotsData
	fetchedAt time.Time
}

var (
	robotsMu    sync.RWMutex
	robotsCache = make(map[string]*robotsEntry)
)

// checkRobots returns true if the given URL is allowed by the site's robots.txt.
// It fetches and caches robots.txt per host. If fetching fails, it allows the URL.
func checkRobots(pageURL string) (bool, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return false, fmt.Errorf("parse url: %w", err)
	}

	host := u.Scheme + "://" + u.Host
	path := u.Path
	if path == "" {
		path = "/"
	}

	robotsMu.RLock()
	entry, ok := robotsCache[host]
	robotsMu.RUnlock()

	if !ok || time.Since(entry.fetchedAt) > robotsCacheTTL {
		data, err := fetchRobotsTxt(host)
		if err != nil {
			// If we can't fetch robots.txt, allow by default (fail open)
			return true, nil
		}
		entry = &robotsEntry{data: data, fetchedAt: time.Now()}
		robotsMu.Lock()
		robotsCache[host] = entry
		robotsMu.Unlock()
	}

	if entry.data == nil {
		return true, nil
	}

	grp := entry.data.FindGroup(robotsUserAgent)
	return grp.Test(path), nil
}

func fetchRobotsTxt(host string) (*robotstxt.RobotsData, error) {
	robotsURL := host + "/robots.txt"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(robotsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	data, err := robotstxt.FromResponse(resp)
	if err != nil {
		return nil, err
	}
	return data, nil
}
