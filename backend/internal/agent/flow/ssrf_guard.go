package flow

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

var privateBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // loopback
		"::1/128",        // IPv6 loopback
		"10.0.0.0/8",     // RFC1918 A
		"172.16.0.0/12",  // RFC1918 B
		"192.168.0.0/16", // RFC1918 C
		"169.254.0.0/16", // link-local (AWS metadata)
		"fc00::/7",       // IPv6 ULA
		"fe80::/10",      // IPv6 link-local
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateBlocks = append(privateBlocks, block)
	}
}

// CheckURL returns nil if the URL is safe to fetch given allowPrivate.
func CheckURL(rawURL string, allowPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https schemes allowed (got %q)", u.Scheme)
	}
	host := u.Hostname()
	if allowPrivate {
		return nil
	}
	// Reject literal localhost
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("private host blocked: localhost")
	}
	// Resolve and check each IP
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}
	for _, ip := range ips {
		for _, block := range privateBlocks {
			if block.Contains(ip) {
				return fmt.Errorf("private IP blocked: %s (%s)", host, ip)
			}
		}
	}
	return nil
}
