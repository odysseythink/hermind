// tool/mcp/transport_helpers.go
package mcp

import (
	"io"
	"os"
	"strings"
	"time"
)

// osEnvironSnapshot returns a snapshot of os.Environ() for the subprocess.
func osEnvironSnapshot() []string {
	return os.Environ()
}

// timeoutChan returns a channel that fires after 2 seconds.
// Used as a simple shutdown grace period.
func timeoutChan() <-chan time.Time {
	return time.After(2 * time.Second)
}

// isClosedPipeError reports whether err represents a closed pipe or closed file
// descriptor, which should be treated as io.EOF by Recv.
func isClosedPipeError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF || err == io.ErrClosedPipe {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "file already closed") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "read/write on closed pipe")
}
