package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

var (
	ErrNoSubscription      = errors.New("no push subscription for user")
	ErrInvalidSubscription = errors.New("invalid subscription")
)

const (
	settingVAPIDPub  = "webpush_vapid_public_key"
	settingVAPIDPriv = "webpush_vapid_private_key"
)

type WebPushOptions struct {
	MailTo     string       // RFC 8292 Subscriber, e.g. "mailto:admin@example.com"
	TTL        int          // seconds; default 60
	HTTPClient *http.Client // optional custom HTTP client for push delivery
}

func (o *WebPushOptions) fill() {
	if o.TTL <= 0 {
		o.TTL = 60
	}
	if o.MailTo == "" {
		o.MailTo = "mailto:webpush@hermind.local"
	}
}

type WebPushPayload struct {
	Title   string              `json:"title"`
	Body    string              `json:"body"`
	Data    map[string]any      `json:"data,omitempty"`
	Actions []map[string]string `json:"actions,omitempty"`
	Image   string              `json:"image,omitempty"`
}

type WebPushService struct {
	db   *gorm.DB
	sys  *SystemService
	enc  *utils.EncryptionManager
	opts WebPushOptions

	mu        sync.RWMutex
	vapidPub  string
	vapidPriv string
	subs      sync.Map // userID(int) -> *webpush.Subscription

	bootOnce sync.Once
}

func NewWebPushService(db *gorm.DB, sys *SystemService, enc *utils.EncryptionManager, opts WebPushOptions) *WebPushService {
	opts.fill()
	return &WebPushService{db: db, sys: sys, enc: enc, opts: opts}
}

func (s *WebPushService) PublicVAPIDKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vapidPub
}

func (s *WebPushService) HasSubscription(userID int) (*webpush.Subscription, bool) {
	v, ok := s.subs.Load(userID)
	if !ok {
		return nil, false
	}
	sub, ok := v.(*webpush.Subscription)
	return sub, ok
}

// Init bootstraps VAPID keys (generates on first run) and loads existing
// subscriptions from the users table into the in-memory cache.
func (s *WebPushService) Init(ctx context.Context) error {
	pubEnc, _ := s.sys.GetSetting(ctx, settingVAPIDPub)
	privEnc, _ := s.sys.GetSetting(ctx, settingVAPIDPriv)
	var pub, priv string
	var err error
	if pubEnc != "" && privEnc != "" {
		pub, err = s.enc.Decrypt(pubEnc)
		if err == nil {
			priv, err = s.enc.Decrypt(privEnc)
		}
	}
	if err != nil || pub == "" || priv == "" {
		priv, pub, err = webpush.GenerateVAPIDKeys()
		if err != nil {
			return err
		}
		encPub, err := s.enc.Encrypt(pub)
		if err != nil {
			return err
		}
		encPriv, err := s.enc.Encrypt(priv)
		if err != nil {
			return err
		}
		if err := s.sys.SetSetting(ctx, settingVAPIDPub, encPub); err != nil {
			return err
		}
		if err := s.sys.SetSetting(ctx, settingVAPIDPriv, encPriv); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.vapidPub = pub
	s.vapidPriv = priv
	s.mu.Unlock()

	return s.loadSubscriptions(ctx)
}

func (s *WebPushService) loadSubscriptions(ctx context.Context) error {
	var users []models.User
	if err := s.db.WithContext(ctx).
		Where("web_push_subscription_config IS NOT NULL").
		Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		if u.WebPushSubscriptionConfig == nil {
			continue
		}
		plain, err := s.enc.Decrypt(*u.WebPushSubscriptionConfig)
		if err != nil {
			mlog.Warning("webpush: decrypt subscription failed", mlog.Int("user", u.ID), mlog.Err(err))
			continue
		}
		var sub webpush.Subscription
		if err := json.Unmarshal([]byte(plain), &sub); err != nil {
			mlog.Warning("webpush: unmarshal subscription failed", mlog.Int("user", u.ID), mlog.Err(err))
			continue
		}
		s.subs.Store(u.ID, &sub)
	}
	return nil
}

// RegisterSubscription stores an encrypted subscription on the user row and
// caches it for fast lookup. subJSON is the JSON body the client received from
// the browser's PushManager.subscribe() call.
func (s *WebPushService) RegisterSubscription(ctx context.Context, userID int, subJSON []byte) error {
	var sub webpush.Subscription
	if err := json.Unmarshal(subJSON, &sub); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSubscription, err)
	}
	if sub.Endpoint == "" {
		return fmt.Errorf("%w: subscription endpoint is required", ErrInvalidSubscription)
	}
	if sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		return fmt.Errorf("%w: subscription keys p256dh and auth are required", ErrInvalidSubscription)
	}
	if err := validatePushEndpointFn(sub.Endpoint); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSubscription, err)
	}
	encrypted, err := s.enc.Encrypt(string(subJSON))
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Update("web_push_subscription_config", &encrypted).Error; err != nil {
		return err
	}
	s.subs.Store(userID, &sub)
	return nil
}

// Send dispatches a notification to one user. Returns ErrNoSubscription
// silently when the user has no registered endpoint; treats 404/410 from
// the push provider as "subscription gone" and removes the cache + DB row.
func (s *WebPushService) Send(ctx context.Context, userID int, payload WebPushPayload) error {
	sub, ok := s.HasSubscription(userID)
	if !ok {
		return ErrNoSubscription
	}
	body, _ := json.Marshal(payload)
	s.mu.RLock()
	pub, priv, mailTo, ttl := s.vapidPub, s.vapidPriv, s.opts.MailTo, s.opts.TTL
	s.mu.RUnlock()

	resp, err := webpush.SendNotificationWithContext(ctx, body, sub, &webpush.Options{
		Subscriber:      mailTo,
		VAPIDPublicKey:  pub,
		VAPIDPrivateKey: priv,
		TTL:             ttl,
		HTTPClient:      s.opts.HTTPClient,
	})
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
		s.subs.Delete(userID)
		if err := s.db.WithContext(ctx).Model(&models.User{}).
			Where("id = ?", userID).Update("web_push_subscription_config", nil).Error; err != nil {
			mlog.Warning("webpush: failed to clear stale subscription", mlog.Int("user", userID), mlog.Err(err))
		}
	}
	return nil
}

// Boot registers handlers for scheduled-job events. Called once at server
// startup; subsequent calls are no-ops.
func (s *WebPushService) Boot(eventLog *EventLogService) {
	s.bootOnce.Do(func() {
		eventLog.Subscribe("scheduled_job_completed", s.onJobCompleted)
		eventLog.Subscribe("scheduled_job_failed", s.onJobFailed)
		eventLog.Subscribe("scheduled_job_timed_out", s.onJobFailed) // same payload
	})
}

func (s *WebPushService) onJobCompleted(ctx context.Context, e EventEnvelope) {
	jobID, _ := pickInt(e.Metadata, "jobId")
	runID, _ := pickInt(e.Metadata, "runId")
	if e.UserID == nil {
		// Phase 2 jobs are not user-owned; nothing to deliver.
		return
	}
	body := truncate(stringFromMeta(e.Metadata, "resultText"), 200)
	if body == "" {
		body = "Your scheduled job finished."
	}
	if err := s.Send(ctx, *e.UserID, WebPushPayload{
		Title: "Scheduled job finished",
		Body:  body,
		Data: map[string]any{
			"onClickUrl": "/workspace/scheduled-jobs?run=" + itoa(runID),
			"jobId":      jobID, "runId": runID,
		},
	}); err != nil && !errors.Is(err, ErrNoSubscription) {
		mlog.Warning("webpush: send failed", mlog.Int("user", *e.UserID), mlog.Err(err))
	}
}

func (s *WebPushService) onJobFailed(ctx context.Context, e EventEnvelope) {
	if e.UserID == nil {
		return
	}
	msg := stringFromMeta(e.Metadata, "error")
	if msg == "" {
		msg = "Scheduled job failed."
	}
	runID, _ := pickInt(e.Metadata, "runId")
	if err := s.Send(ctx, *e.UserID, WebPushPayload{
		Title: "Scheduled job failed",
		Body:  truncate(msg, 200),
		Data: map[string]any{
			"onClickUrl": "/workspace/scheduled-jobs?run=" + itoa(runID),
			"runId":      runID,
		},
	}); err != nil && !errors.Is(err, ErrNoSubscription) {
		mlog.Warning("webpush: send failed", mlog.Int("user", *e.UserID), mlog.Err(err))
	}
}

func pickInt(m map[string]any, key string) (int, bool) {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n, true
		case int64:
			return int(n), true
		case float64:
			return int(math.Round(n)), true
		}
	}
	return 0, false
}

func stringFromMeta(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// validatePushEndpoint ensures the endpoint is a public HTTPS URL suitable for
// web push delivery. It rejects non-HTTPS schemes, localhost, and private IPs
// to prevent SSRF.
// validatePushEndpointFn is overridable in tests to bypass DNS/SSRF checks.
var validatePushEndpointFn = validatePushEndpoint

func validatePushEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https scheme allowed (got %q)", u.Scheme)
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("private host blocked: localhost")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("private IP blocked: %s", host)
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
		}
		for _, ip := range ips {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				return fmt.Errorf("private IP blocked: %s (%s)", host, ip)
			}
		}
	}
	return nil
}
