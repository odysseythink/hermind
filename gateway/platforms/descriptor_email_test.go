package platforms

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// startFakeSMTP starts a tiny SMTP server that announces EHLO, accepts
// AUTH LOGIN (ignoring the credentials), and replies 221 to QUIT. It
// returns the listener's "host:port" so tests can point testEmail at it.
func startFakeSMTP(t *testing.T, rejectAuth bool) (hostPort string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		_, _ = rw.WriteString("220 fake.example.com ESMTP ready\r\n")
		_ = rw.Flush()
		for {
			line, err := rw.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				_, _ = rw.WriteString("250-fake.example.com hello\r\n")
				_, _ = rw.WriteString("250 AUTH PLAIN LOGIN\r\n")
				_ = rw.Flush()
			case strings.HasPrefix(line, "AUTH PLAIN"):
				if rejectAuth {
					_, _ = rw.WriteString("535 5.7.8 authentication failed\r\n")
					_ = rw.Flush()
					return
				}
				_, _ = rw.WriteString("235 2.7.0 Authentication succeeded\r\n")
				_ = rw.Flush()
			case strings.HasPrefix(line, "AUTH LOGIN"):
				if rejectAuth {
					_, _ = rw.WriteString("535 5.7.8 authentication failed\r\n")
					_ = rw.Flush()
					return
				}
				_, _ = rw.WriteString("334 VXNlcm5hbWU6\r\n") // "Username:"
				_ = rw.Flush()
				_, _ = rw.ReadString('\n') // user b64
				_, _ = rw.WriteString("334 UGFzc3dvcmQ6\r\n") // "Password:"
				_ = rw.Flush()
				_, _ = rw.ReadString('\n') // pass b64
				_, _ = rw.WriteString("235 2.7.0 Authentication succeeded\r\n")
				_ = rw.Flush()
			case strings.HasPrefix(line, "QUIT"):
				_, _ = rw.WriteString("221 2.0.0 Bye\r\n")
				_ = rw.Flush()
				return
			case strings.HasPrefix(line, "NOOP"):
				_, _ = rw.WriteString("250 2.0.0 OK\r\n")
				_ = rw.Flush()
			default:
				_, _ = rw.WriteString("502 5.5.1 unknown\r\n")
				_ = rw.Flush()
			}
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close(); <-done }
}

func TestEmail_SuccessWithAuth(t *testing.T) {
	hostPort, stop := startFakeSMTP(t, false)
	defer stop()
	host, port, _ := net.SplitHostPort(hostPort)

	if err := testEmail(context.Background(), host, port, "u", "p"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestEmail_SuccessNoAuth(t *testing.T) {
	hostPort, stop := startFakeSMTP(t, false)
	defer stop()
	host, port, _ := net.SplitHostPort(hostPort)

	if err := testEmail(context.Background(), host, port, "", ""); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestEmail_AuthRejected(t *testing.T) {
	hostPort, stop := startFakeSMTP(t, true)
	defer stop()
	host, port, _ := net.SplitHostPort(hostPort)

	if err := testEmail(context.Background(), host, port, "u", "p"); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestEmail_MissingHost(t *testing.T) {
	if err := testEmail(context.Background(), "", "587", "", ""); err == nil {
		t.Error("expected error for empty host")
	}
}
