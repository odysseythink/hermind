package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// Matrix is an outbound-only Matrix client-server API adapter. It
// sends m.room.message text events to a single configured room.
// Inbound sync is not implemented in Plan 7b.1.
type Matrix struct {
	HomeServer  string // e.g. https://matrix.example.com
	AccessToken string
	RoomID      string // !abc:example.com — default room when OutgoingMessage.ChatID is empty
	client      *http.Client
	txnCounter  int64
}

func NewMatrix(homeServer, accessToken, roomID string) *Matrix {
	return &Matrix{
		HomeServer:  strings.TrimRight(homeServer, "/"),
		AccessToken: accessToken,
		RoomID:      roomID,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *Matrix) Name() string { return "matrix" }

func (m *Matrix) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

// nextTxnID returns a unique, monotonically-increasing transaction ID
// for PUT idempotency. Callers combine it with a unix-nanos prefix.
func (m *Matrix) nextTxnID() string {
	n := atomic.AddInt64(&m.txnCounter, 1)
	return strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatInt(n, 10)
}

func (m *Matrix) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if m.HomeServer == "" || m.AccessToken == "" {
		return fmt.Errorf("matrix: home_server/access_token required")
	}
	roomID := m.RoomID
	if out.ChatID != "" {
		roomID = out.ChatID
	}
	if roomID == "" {
		return fmt.Errorf("matrix: room id required (config or OutgoingMessage.ChatID)")
	}
	txn := m.nextTxnID()
	endpoint := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		m.HomeServer, url.PathEscape(roomID), url.PathEscape(txn))
	payload := map[string]any{
		"msgtype": "m.text",
		"body":    out.Text,
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "PUT", endpoint, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.AccessToken)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("matrix: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("matrix: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
