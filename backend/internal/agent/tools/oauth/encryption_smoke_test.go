package oauth_test

import (
	"testing"

	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestEncryptionManager_RoundTripFromOauthPackage(t *testing.T) {
	enc, err := utils.NewEncryptionManager(t.TempDir())
	require.NoError(t, err)
	const plain = "OAQABAAAAAAA-fake-refresh-token-shape"
	ct, err := enc.Encrypt(plain)
	require.NoError(t, err)
	require.NotEqual(t, plain, ct)
	pt, err := enc.Decrypt(ct)
	require.NoError(t, err)
	require.Equal(t, plain, pt)
}
