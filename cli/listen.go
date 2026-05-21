package cli

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"strings"
	"syscall"
)

const (
	defaultPort  = 36265
	portMin      = 30000
	portMax      = 40000
	portAttempts = 50
)

// listenDefaultPort first tries the fixed default port (36265) on
// 127.0.0.1. If it is already in use, falls back to a random port in
// [portMin, portMax). Any other bind error fails immediately.
func listenDefaultPort() (net.Listener, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", defaultPort))
	if err == nil {
		return ln, nil
	}
	if isAddrInUse(err) {
		return listenOnRange(portMin, portMax, portAttempts)
	}
	return nil, fmt.Errorf("listen: %w", err)
}

// listenRandomLocalhost picks a random TCP port in [portMin, portMax)
// on 127.0.0.1 and returns the bound listener. Retries up to
// portAttempts times on EADDRINUSE before giving up. Any other bind
// error fails immediately.
func listenRandomLocalhost() (net.Listener, error) {
	return listenOnRange(portMin, portMax, portAttempts)
}

// listenOnRange is the underlying helper, split out for testability.
func listenOnRange(minPort, maxPort, attempts int) (net.Listener, error) {
	if minPort <= 0 || maxPort <= minPort {
		return nil, fmt.Errorf("listen: invalid port range [%d,%d)", minPort, maxPort)
	}
	span := maxPort - minPort
	for i := 0; i < attempts; i++ {
		port := minPort + rand.IntN(span)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return ln, nil
		}
		if !isAddrInUse(err) {
			return nil, fmt.Errorf("listen: %w", err)
		}
	}
	return nil, fmt.Errorf("listen: no free localhost port in [%d,%d) after %d attempts",
		minPort, maxPort, attempts)
}

func isAddrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	// Windows returns a different error structure; fall back to string
	// matching for cross-platform compatibility.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage of each socket address")
}
