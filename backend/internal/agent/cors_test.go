package agent

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildCheckOrigin_Empty_AllowsSameHost(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: ""}
	check := buildCheckOrigin(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com:3001")
	req.Host = "example.com:3001"
	require.True(t, check(req))

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Origin", "https://evil.com")
	req2.Host = "example.com:3001"
	require.False(t, check(req2))
}

func TestBuildCheckOrigin_Wildcard_AllowsAny(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: "*"}
	check := buildCheckOrigin(cfg)

	require.True(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://anything.com"}}}))
	require.True(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://elsewhere.org"}}}))
}

func TestBuildCheckOrigin_CSV_MatchesExactOrigin(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: "https://a.com,https://b.com"}
	check := buildCheckOrigin(cfg)

	require.True(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://a.com"}}}))
	require.True(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://b.com"}}}))
	require.False(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://c.com"}}}))
}

func TestBuildCheckOrigin_NoOriginHeader_Allows(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: ""}
	check := buildCheckOrigin(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"
	require.True(t, check(req))
}

func TestCheckOrigin_WildcardSuffix_MatchesSubdomain(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: "*.example.com"}
	check := buildCheckOrigin(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://a.example.com")
	require.True(t, check(req))

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Origin", "https://x.y.example.com")
	require.True(t, check(req2))
}

func TestCheckOrigin_WildcardSuffix_DoesNotMatchPrefixInjection(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: "*.example.com"}
	check := buildCheckOrigin(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil-example.com")
	require.False(t, check(req))
}

func TestCheckOrigin_WildcardSuffix_DoesNotMatchBareDomain(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: "*.example.com"}
	check := buildCheckOrigin(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	require.False(t, check(req))
}

func TestSession_NoTODOConnField_StructDoesNotHaveIt(t *testing.T) {
	var s Session
	typ := reflect.TypeOf(&s).Elem()
	_, hasConn := typ.FieldByName("conn")
	require.False(t, hasConn, "Session should not have a 'conn' field (PR-AR-5 cleanup)")
}

func TestCheckOrigin_MixedExactAndWildcard(t *testing.T) {
	cfg := &config.Config{AgentAllowedOrigins: "https://a.com,*.b.com"}
	check := buildCheckOrigin(cfg)

	require.True(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://a.com"}}}))
	require.True(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://sub.b.com"}}}))
	require.False(t, check(&http.Request{Header: http.Header{"Origin": []string{"https://c.com"}}}))
}
