package oauth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeState_RoundTrip(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-only")
	payload := oauth.StatePayload{
		UserID:    42,
		Nonce:     "abc123",
		ReturnTo:  "https://app.example.com/dashboard",
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}
	encoded := oauth.EncodeState(secret, payload)
	decoded, err := oauth.DecodeState(secret, encoded, "https://app.example.com")
	require.NoError(t, err)
	require.Equal(t, payload.UserID, decoded.UserID)
	require.Equal(t, payload.Nonce, decoded.Nonce)
	require.Equal(t, payload.ReturnTo, decoded.ReturnTo)
	require.Equal(t, payload.ExpiresAt, decoded.ExpiresAt)
}

func TestDecodeState_TamperedNonce_Rejected(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-only")
	payload := oauth.StatePayload{
		UserID:    42,
		Nonce:     "abc123",
		ReturnTo:  "https://app.example.com/dashboard",
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}
	encoded := oauth.EncodeState(secret, payload)
	parts := strings.SplitN(encoded, ".", 2)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)

	var p oauth.StatePayload
	require.NoError(t, json.Unmarshal(raw, &p))
	p.Nonce = "tampered"
	newRaw, _ := json.Marshal(p)
	// Keep the original signature — tampered payload with old signature.
	newEncoded := base64.RawURLEncoding.EncodeToString(newRaw) + "." + parts[1]

	_, err = oauth.DecodeState(secret, newEncoded, "https://app.example.com")
	require.Error(t, err)
	require.True(t, errors.Is(err, oauth.ErrStateInvalid), "expected ErrStateInvalid, got %v", err)
}

func TestDecodeState_TamperedReturnTo_Rejected(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-only")
	payload := oauth.StatePayload{
		UserID:    42,
		Nonce:     "abc123",
		ReturnTo:  "https://app.example.com/dashboard",
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}
	encoded := oauth.EncodeState(secret, payload)
	parts := strings.SplitN(encoded, ".", 2)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)

	var p oauth.StatePayload
	require.NoError(t, json.Unmarshal(raw, &p))
	p.ReturnTo = "https://evil.com/phish"
	newRaw, _ := json.Marshal(p)
	mac := hmac.New(sha256.New, secret)
	mac.Write(newRaw)
	newSig := mac.Sum(nil)
	newEncoded := base64.RawURLEncoding.EncodeToString(newRaw) + "." + base64.RawURLEncoding.EncodeToString(newSig)

	_, err = oauth.DecodeState(secret, newEncoded, "https://app.example.com")
	require.Error(t, err)
	require.True(t, errors.Is(err, oauth.ErrStateRedirect), "expected ErrStateRedirect, got %v", err)
}

func TestDecodeState_Expired_Rejected(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-only")
	payload := oauth.StatePayload{
		UserID:    42,
		Nonce:     "abc123",
		ReturnTo:  "https://app.example.com/dashboard",
		ExpiresAt: time.Now().Add(-1 * time.Second).Unix(),
	}
	encoded := oauth.EncodeState(secret, payload)
	_, err := oauth.DecodeState(secret, encoded, "https://app.example.com")
	require.Error(t, err)
	require.True(t, errors.Is(err, oauth.ErrStateExpired), "expected ErrStateExpired, got %v", err)
}

func TestDecodeState_OpenRedirect_Rejected(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-only")
	payload := oauth.StatePayload{
		UserID:    42,
		Nonce:     "abc123",
		ReturnTo:  "https://evil.com/phish",
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}
	encoded := oauth.EncodeState(secret, payload)
	_, err := oauth.DecodeState(secret, encoded, "https://app.example.com")
	require.Error(t, err)
	require.True(t, errors.Is(err, oauth.ErrStateRedirect), "expected ErrStateRedirect, got %v", err)
}

func TestDecodeState_OpenRedirect_SubdomainBoundary_Rejected(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-only")
	payload := oauth.StatePayload{
		UserID:    42,
		Nonce:     "abc123",
		ReturnTo:  "https://app.example.com.evil.com/phish",
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}
	encoded := oauth.EncodeState(secret, payload)
	_, err := oauth.DecodeState(secret, encoded, "https://app.example.com")
	require.Error(t, err)
	require.True(t, errors.Is(err, oauth.ErrStateRedirect), "expected ErrStateRedirect, got %v", err)
}

func TestDecodeState_MalformedBase64_Rejected(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-only")
	_, err := oauth.DecodeState(secret, "not-valid-base64!!!", "https://app.example.com")
	require.Error(t, err)
	require.True(t, errors.Is(err, oauth.ErrStateInvalid), "expected ErrStateInvalid, got %v", err)
}
