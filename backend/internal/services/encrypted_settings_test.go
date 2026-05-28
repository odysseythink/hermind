package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newEncTestEnv(t *testing.T) (*SystemService, *utils.EncryptionManager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}))
	enc, err := utils.NewEncryptionManager(t.TempDir())
	require.NoError(t, err)
	return NewSystemService(db), enc
}

func TestGetSecretField_PlainValue_ReturnsAsIs(t *testing.T) {
	sysSvc, enc := newEncTestEnv(t)
	ctx := context.Background()
	require.NoError(t, sysSvc.SetSetting(ctx, "test_config", `{"clientSecret":"plain-secret"}`))

	val, err := sysSvc.GetSecretField(ctx, "test_config", "clientSecret", enc)
	require.NoError(t, err)
	require.Equal(t, "plain-secret", val)
}

func TestGetSecretField_EncPrefix_Decrypts(t *testing.T) {
	sysSvc, enc := newEncTestEnv(t)
	ctx := context.Background()

	ciphertext, err := enc.Encrypt("my-secret")
	require.NoError(t, err)
	require.NoError(t, sysSvc.SetSetting(ctx, "test_config", `{"clientSecret":"enc:`+ciphertext+`"}`))

	val, err := sysSvc.GetSecretField(ctx, "test_config", "clientSecret", enc)
	require.NoError(t, err)
	require.Equal(t, "my-secret", val)
}

func TestSetSecretField_EncryptsBeforeSave(t *testing.T) {
	sysSvc, enc := newEncTestEnv(t)
	ctx := context.Background()
	require.NoError(t, sysSvc.SetSetting(ctx, "test_config", `{"clientId":"my-client"}`))

	require.NoError(t, sysSvc.SetSecretField(ctx, "test_config", "clientSecret", "shhh", enc))

	raw, err := sysSvc.GetSetting(ctx, "test_config")
	require.NoError(t, err)
	require.Contains(t, raw, `"clientSecret":"enc:`)
}

func TestSetSecretField_RoundTrip_ViaGet(t *testing.T) {
	sysSvc, enc := newEncTestEnv(t)
	ctx := context.Background()

	require.NoError(t, sysSvc.SetSecretField(ctx, "test_config", "apiKey", "round-trip-key", enc))
	val, err := sysSvc.GetSecretField(ctx, "test_config", "apiKey", enc)
	require.NoError(t, err)
	require.Equal(t, "round-trip-key", val)
}

func TestSetSecretField_OtherFieldsPreserved(t *testing.T) {
	sysSvc, enc := newEncTestEnv(t)
	ctx := context.Background()
	require.NoError(t, sysSvc.SetSetting(ctx, "test_config", `{"clientId":"my-client","clientSecret":"old"}`))

	require.NoError(t, sysSvc.SetSecretField(ctx, "test_config", "clientSecret", "new-secret", enc))

	raw, err := sysSvc.GetSetting(ctx, "test_config")
	require.NoError(t, err)
	require.Contains(t, raw, `"clientId":"my-client"`)
	require.Contains(t, raw, `"clientSecret":"enc:`)
}

func TestGetSecretField_MissingKey_ReturnsEmpty(t *testing.T) {
	sysSvc, enc := newEncTestEnv(t)
	ctx := context.Background()

	val, err := sysSvc.GetSecretField(ctx, "missing_config", "clientSecret", enc)
	require.NoError(t, err)
	require.Empty(t, val)
}
