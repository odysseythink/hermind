// provider/zhipu/auth.go
package zhipu

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// signJWT generates a short-lived HS256 JWT for Zhipu AI's Bearer auth scheme.
// The API key format is "<key_id>.<secret>" — the secret is used as the
// HMAC key, and the key_id is embedded in the payload as "api_key".
//
// Reference: https://open.bigmodel.cn/dev/api#http_auth
func signJWT(apiKey string, ttl time.Duration) (string, error) {
	parts := strings.SplitN(apiKey, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("zhipu: api_key must be '<key_id>.<secret>'")
	}
	keyID := parts[0]
	secret := parts[1]

	now := time.Now().UnixMilli()
	exp := now + ttl.Milliseconds()

	header := map[string]any{
		"alg":       "HS256",
		"typ":       "JWT",
		"sign_type": "SIGN",
	}
	payload := map[string]any{
		"api_key":   keyID,
		"exp":       exp,
		"timestamp": now,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := headerB64 + "." + payloadB64

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sig, nil
}
