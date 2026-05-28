package flow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSRFGuard_PublicIP_Allowed(t *testing.T) {
	err := CheckURL("http://8.8.8.8/", false)
	require.NoError(t, err)
}

func TestSSRFGuard_Localhost_Blocked(t *testing.T) {
	err := CheckURL("http://localhost/admin", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "localhost")
}

func TestSSRFGuard_127001_Blocked(t *testing.T) {
	err := CheckURL("http://127.0.0.1/admin", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestSSRFGuard_169254_AWSMetadata_Blocked(t *testing.T) {
	err := CheckURL("http://169.254.169.254/latest/meta-data/", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestSSRFGuard_10dot_PrivateA_Blocked(t *testing.T) {
	err := CheckURL("http://10.0.0.1/", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestSSRFGuard_172_16to31_PrivateB_Blocked(t *testing.T) {
	err := CheckURL("http://172.16.0.1/", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestSSRFGuard_192168_PrivateC_Blocked(t *testing.T) {
	err := CheckURL("http://192.168.1.1/", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestSSRFGuard_IPv6Loopback_Blocked(t *testing.T) {
	err := CheckURL("http://[::1]/", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestSSRFGuard_AllowOverride_PermitsPrivate(t *testing.T) {
	err := CheckURL("http://127.0.0.1/", true)
	require.NoError(t, err)
}

func TestSSRFGuard_InvalidURL_Blocked(t *testing.T) {
	err := CheckURL("://not-a-url", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid URL")
}

func TestSSRFGuard_NonHTTPScheme_Blocked(t *testing.T) {
	err := CheckURL("ftp://example.com/", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only http/https")
}
