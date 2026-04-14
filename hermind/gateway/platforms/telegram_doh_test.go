package platforms

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoHResolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-json")
		fmt.Fprint(w, `{"Answer":[{"type":1,"data":"1.2.3.4"}]}`)
	}))
	defer srv.Close()

	ips, err := dohResolve(context.Background(), srv.URL, "example.com")
	if err != nil {
		t.Fatalf("dohResolve error: %v", err)
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Fatalf("expected [1.2.3.4], got %v", ips)
	}
}

func TestDoHResolveMergesProviders(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-json")
		fmt.Fprint(w, `{"Answer":[{"type":1,"data":"10.0.0.1"}]}`)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-json")
		fmt.Fprint(w, `{"Answer":[{"type":1,"data":"10.0.0.2"}]}`)
	}))
	defer srv2.Close()

	ips := discoverFallbackIPs(context.Background(), "example.com", []string{srv1.URL, srv2.URL})
	if len(ips) < 2 {
		t.Fatalf("expected >= 2 IPs, got %v", ips)
	}
}

type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")}
}

func TestDoHTransportFallsBack(t *testing.T) {
	var received bool
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Extract host:port from the test server URL.
	addr := backend.Listener.Addr().String()

	req, err := http.NewRequest(http.MethodGet, "http://api.telegram.org/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	dt := &DoHTransport{
		FallbackIPs: []string{addr},
		Primary:     &failingPrimaryTransport{fallbackAddr: addr},
	}

	resp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	resp.Body.Close()

	if !received {
		t.Fatal("fallback server did not receive the request")
	}
}

// failingPrimaryTransport fails with a net.OpError for the original host
// but delegates to http.DefaultTransport for fallback IPs.
type failingPrimaryTransport struct {
	fallbackAddr string
}

func (f *failingPrimaryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == f.fallbackAddr {
		return http.DefaultTransport.RoundTrip(req)
	}
	return nil, &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")}
}
