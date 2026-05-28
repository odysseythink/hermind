package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrStateInvalid  = errors.New("invalid state")
	ErrStateExpired  = errors.New("state expired")
	ErrStateRedirect = errors.New("state return_to outside trusted prefix")
)

type StatePayload struct {
	UserID    int    `json:"u"`
	Nonce     string `json:"n"`
	ReturnTo  string `json:"r"`
	ExpiresAt int64  `json:"e"` // unix seconds
}

func EncodeState(secret []byte, p StatePayload) string {
	raw, _ := json.Marshal(p)
	mac := hmac.New(sha256.New, secret)
	mac.Write(raw)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(raw) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
}

func DecodeState(secret []byte, encoded, publicBaseURL string) (*StatePayload, error) {
	parts := strings.SplitN(encoded, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: malformed", ErrStateInvalid)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: bad base64 payload", ErrStateInvalid)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: bad base64 signature", ErrStateInvalid)
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(raw)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, fmt.Errorf("%w: signature mismatch", ErrStateInvalid)
	}
	var p StatePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("%w: bad json", ErrStateInvalid)
	}
	if time.Now().Unix() > p.ExpiresAt {
		return nil, ErrStateExpired
	}
	if p.ReturnTo != publicBaseURL && !strings.HasPrefix(p.ReturnTo, publicBaseURL+"/") {
		return nil, ErrStateRedirect
	}
	return &p, nil
}
