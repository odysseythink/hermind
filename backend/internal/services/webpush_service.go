package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

var ErrNoSubscription = errors.New("no push subscription for user")

const (
	settingVAPIDPub  = "webpush_vapid_public_key"
	settingVAPIDPriv = "webpush_vapid_private_key"
)

type WebPushOptions struct {
	MailTo string // RFC 8292 Subscriber, e.g. "mailto:admin@example.com"
	TTL    int    // seconds; default 60
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
	return v.(*webpush.Subscription), true
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
		return err
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

	resp, err := webpush.SendNotification(body, sub, &webpush.Options{
		Subscriber:      mailTo,
		VAPIDPublicKey:  pub,
		VAPIDPrivateKey: priv,
		TTL:             ttl,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
		s.subs.Delete(userID)
		_ = s.db.WithContext(ctx).Model(&models.User{}).
			Where("id = ?", userID).Update("web_push_subscription_config", nil).Error
	}
	return nil
}
