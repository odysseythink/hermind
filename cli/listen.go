package cli

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"syscall"
)

const (
	portMin      = 30000
	portMax      = 40000
	portAttempts = 50
)

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
	return errors.Is(err, syscall.EADDRINUSE)
}
