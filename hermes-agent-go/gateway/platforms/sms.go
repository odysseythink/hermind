package platforms

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// SMS is a Twilio REST adapter. It uses Basic Auth (account SID +
// auth token) and form-encoded POST to Messages.json. Inbound SMS
// (Twilio webhook) is not supported in Plan 7b — pair with api_server
// and let Twilio forward inbound webhooks to it externally.
type SMS struct {
	AccountSID string
	AuthToken  string
	From       string
	To         string
	BaseURL    string // overridable for tests, defaults to Twilio prod
	client     *http.Client
}

// NewSMS constructs a Twilio SMS adapter.
func NewSMS(accountSID, authToken, from, to string) *SMS {
	return &SMS{
		AccountSID: accountSID,
		AuthToken:  authToken,
		From:       from,
		To:         to,
		BaseURL:    "https://api.twilio.com",
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

// WithBaseURL overrides the Twilio base URL (used in tests).
func (s *SMS) WithBaseURL(u string) *SMS {
	s.BaseURL = strings.TrimRight(u, "/")
	return s
}

func (s *SMS) Name() string { return "sms" }

func (s *SMS) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (s *SMS) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if s.AccountSID == "" || s.AuthToken == "" || s.From == "" || s.To == "" {
		return fmt.Errorf("sms: account_sid/auth_token/from/to are required")
	}
	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", s.BaseURL, s.AccountSID)
	form := url.Values{}
	form.Set("From", s.From)
	form.Set("To", s.To)
	form.Set("Body", out.Text)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.AccountSID, s.AuthToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sms: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("sms: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
