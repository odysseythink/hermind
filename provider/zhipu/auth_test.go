// provider/zhipu/auth_test.go
package zhipu

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignJWTFormat(t *testing.T) {
	token, err := signJWT("my_key.my_secret", time.Hour)
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "JWT must have 3 parts")

	// Header
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var hdr map[string]any
	require.NoError(t, json.Unmarshal(hdrBytes, &hdr))
	assert.Equal(t, "HS256", hdr["alg"])
	assert.Equal(t, "JWT", hdr["typ"])
	assert.Equal(t, "SIGN", hdr["sign_type"])

	// Payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadBytes, &payload))
	assert.Equal(t, "my_key", payload["api_key"])
	// exp and timestamp should be present and numeric
	assert.Contains(t, payload, "exp")
	assert.Contains(t, payload, "timestamp")
}

func TestSignJWTRejectsMalformedKey(t *testing.T) {
	_, err := signJWT("no_dot_in_this_key", time.Hour)
	assert.Error(t, err)
}

func TestSignJWTSecretAffectsSignature(t *testing.T) {
	a, _ := signJWT("k.secret_a", time.Hour)
	b, _ := signJWT("k.secret_b", time.Hour)
	sigA := strings.Split(a, ".")[2]
	sigB := strings.Split(b, ".")[2]
	assert.NotEqual(t, sigA, sigB)
}
