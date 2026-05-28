package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func newTestAuthEnv(t *testing.T) (*AuthService, *SystemService, *AdminService, *config.Config) {
	t.Helper()
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "old-secret", AuthToken: "old-token", MultiUserMode: false}
	db, err := NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, _ := db.DB(); sqlDB != nil {
			sqlDB.Close()
		}
	})
	assert.NoError(t, AutoMigrate(db))
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	return NewAuthService(db, cfg, enc), NewSystemService(db), NewAdminService(db), cfg
}

func TestRotateCredentials_DisablePassword(t *testing.T) {
	auth, sys, _, cfg := newTestAuthEnv(t)
	assert.NoError(t, auth.RotateCredentials(context.Background(), sys, false, ""))
	assert.Equal(t, "", cfg.AuthToken)
	assert.Equal(t, "", cfg.JWTSecret)
	v, _ := sys.GetSetting(context.Background(), "auth_token")
	assert.Equal(t, "", v)
}

func TestRotateCredentials_NewPassword_Persisted(t *testing.T) {
	auth, sys, _, cfg := newTestAuthEnv(t)
	assert.NoError(t, auth.RotateCredentials(context.Background(), sys, true, "newPassw0rd"))
	assert.Equal(t, "newPassw0rd", cfg.AuthToken)
	assert.NotEqual(t, "old-secret", cfg.JWTSecret)
	assert.NotEqual(t, "", cfg.JWTSecret)
	persisted, _ := sys.GetSetting(context.Background(), "auth_token")
	assert.Equal(t, "newPassw0rd", persisted)
}

func TestRotateCredentials_ShortPassword(t *testing.T) {
	auth, sys, _, _ := newTestAuthEnv(t)
	err := auth.RotateCredentials(context.Background(), sys, true, "short")
	assert.Error(t, err)
}

func TestEnableMultiUser_Success(t *testing.T) {
	auth, sys, admin, cfg := newTestAuthEnv(t)
	u, bizErr, sysErr := auth.EnableMultiUser(context.Background(), admin, sys, "newadmin", "newPassw0rd")
	assert.NoError(t, sysErr)
	assert.Equal(t, "", bizErr)
	assert.NotNil(t, u)
	assert.Equal(t, "admin", u.Role)
	assert.True(t, cfg.MultiUserMode)
	flag, _ := sys.GetSetting(context.Background(), "multi_user_mode")
	assert.Equal(t, "true", flag)
}

func TestEnableMultiUser_AlreadyOn(t *testing.T) {
	auth, sys, admin, cfg := newTestAuthEnv(t)
	cfg.MultiUserMode = true
	_, bizErr, sysErr := auth.EnableMultiUser(context.Background(), admin, sys, "x", "y")
	assert.NoError(t, sysErr)
	assert.Equal(t, "Multi-user mode is already enabled.", bizErr)
}

func TestEnableMultiUser_BadUsername(t *testing.T) {
	auth, sys, admin, _ := newTestAuthEnv(t)
	_, bizErr, _ := auth.EnableMultiUser(context.Background(), admin, sys, "A", "newPassw0rd")
	assert.NotEmpty(t, bizErr)
}

func TestUpdateOwnProfile_BioOnly(t *testing.T) {
	auth, _, _, cfg := newTestAuthEnv(t)
	hash, _ := utils.HashPassword("pw")
	u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: "default"}
	assert.NoError(t, auth.db.Create(u).Error)
	bizErr, sysErr := auth.UpdateOwnProfile(context.Background(), u, nil, nil, utils.Ptr("new bio"))
	assert.NoError(t, sysErr)
	assert.Equal(t, "", bizErr)
	var reloaded models.User
	auth.db.First(&reloaded, u.ID)
	assert.Equal(t, "new bio", *reloaded.Bio)
	_ = cfg
}

func TestUpdateOwnProfile_NoUpdates(t *testing.T) {
	auth, _, _, _ := newTestAuthEnv(t)
	hash, _ := utils.HashPassword("pw")
	u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: "default"}
	assert.NoError(t, auth.db.Create(u).Error)
	bizErr, _ := auth.UpdateOwnProfile(context.Background(), u, utils.Ptr("alice"), nil, nil)
	// Username unchanged (same as current) and no other field → empty updates
	assert.Equal(t, "No updates provided", bizErr)
}
