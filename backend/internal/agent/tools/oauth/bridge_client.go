package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBridgeTimeout = 30 * time.Second
	maxBridgeRespBytes   = 4 << 20 // 4 MiB
)

// testBaseURL is overridden by SetTestBaseURL in tests.
var testBaseURL string

// SetTestBaseURL overrides the script.google.com base URL during tests.
// Pass "" to clear. Production callers MUST NOT use this.
func SetTestBaseURL(u string) { testBaseURL = u }

type BridgeClient struct {
	http *http.Client
}

func NewBridgeClient(timeout time.Duration) *BridgeClient {
	if timeout <= 0 {
		timeout = defaultBridgeTimeout
	}
	return &BridgeClient{http: &http.Client{Timeout: timeout}}
}

type bridgeEnvelope struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error"`
}

func (b *BridgeClient) Call(ctx context.Context, deploymentID, apiKey, action string, params map[string]any) (json.RawMessage, error) {
	url := b.endpoint(deploymentID)

	body := make(map[string]any, len(params)+2)
	body["key"] = apiKey
	body["action"] = action
	for k, v := range params {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hermind-UA", "Hermind-Agent-Go/1.0")

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bridge call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bridge HTTP %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBridgeRespBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(raw) > maxBridgeRespBytes {
		return nil, fmt.Errorf("bridge response exceeds %d bytes", maxBridgeRespBytes)
	}

	var env bridgeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("bridge response not JSON: %w", err)
	}
	if env.Status == "error" {
		return nil, fmt.Errorf("apps script error: %s", env.Error)
	}
	return env.Data, nil
}

func (b *BridgeClient) endpoint(deploymentID string) string {
	if testBaseURL != "" {
		return testBaseURL
	}
	return "https://script.google.com/macros/s/" + deploymentID + "/exec"
}
