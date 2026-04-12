package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// dohResponse is the JSON structure returned by DNS-over-HTTPS providers.
type dohResponse struct {
	Answer []dohAnswer `json:"Answer"`
}

// dohAnswer represents a single DNS answer record.
type dohAnswer struct {
	Type int    `json:"type"`
	Data string `json:"data"`
}

// dohResolve queries a single DoH provider for A records of hostname.
func dohResolve(ctx context.Context, providerURL, hostname string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("%s?name=%s&type=A", providerURL, hostname)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var dr dohResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, err
	}

	var ips []string
	for _, ans := range dr.Answer {
		if ans.Type == 1 { // A record
			ips = append(ips, ans.Data)
		}
	}
	return ips, nil
}

// discoverFallbackIPs queries multiple DoH providers concurrently and returns
// deduplicated A-record IPs. Falls back to a seed IP if no results are found.
func discoverFallbackIPs(ctx context.Context, hostname string, providers []string) []string {
	var mu sync.Mutex
	seen := make(map[string]struct{})
	var result []string

	var wg sync.WaitGroup
	for _, p := range providers {
		wg.Add(1)
		go func(provider string) {
			defer wg.Done()
			ips, err := dohResolve(ctx, provider, hostname)
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, ip := range ips {
				if _, ok := seen[ip]; !ok {
					seen[ip] = struct{}{}
					result = append(result, ip)
				}
			}
		}(p)
	}
	wg.Wait()

	if len(result) == 0 {
		return []string{"149.154.167.220"}
	}
	return result
}

// DoHTransport is an http.RoundTripper that falls back to alternative IPs
// discovered via DNS-over-HTTPS when the primary transport fails.
type DoHTransport struct {
	FallbackIPs []string
	Primary     http.RoundTripper
	Timeout     time.Duration

	mu       sync.Mutex
	stickyIP string
}

// RoundTrip implements http.RoundTripper. It tries the primary transport first,
// then falls back to alternative IPs on connection errors.
func (d *DoHTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Try sticky IP first if set.
	d.mu.Lock()
	sticky := d.stickyIP
	d.mu.Unlock()

	if sticky != "" {
		resp, err := d.tryFallbackIP(req, sticky)
		if err == nil {
			return resp, nil
		}
		// Clear sticky on failure.
		d.mu.Lock()
		d.stickyIP = ""
		d.mu.Unlock()
	}

	// Try primary transport.
	resp, err := d.Primary.RoundTrip(req)
	if err == nil {
		return resp, nil
	}
	if !isConnError(err) {
		return nil, err
	}

	// Try each fallback IP.
	for _, ip := range d.FallbackIPs {
		resp, fbErr := d.tryFallbackIP(req, ip)
		if fbErr == nil {
			d.mu.Lock()
			d.stickyIP = ip
			d.mu.Unlock()
			return resp, nil
		}
	}
	return nil, err
}

// tryFallbackIP rewrites the request URL host to the fallback IP while
// preserving the original Host header for TLS SNI.
func (d *DoHTransport) tryFallbackIP(req *http.Request, ip string) (*http.Response, error) {
	clone := req.Clone(req.Context())
	origHost := clone.URL.Host
	clone.URL.Host = ip
	if clone.Host == "" {
		clone.Host = origHost
	}
	transport := d.Primary
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(clone)
}

// isConnError checks whether err is a connection error (*net.OpError).
func isConnError(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return errors.As(urlErr.Err, &opErr)
	}
	return false
}
