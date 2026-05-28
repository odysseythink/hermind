package services

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func testCfg(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
}

func testDB(t *testing.T, cfg *config.Config) *gorm.DB {
	t.Helper()
	db, err := NewDB(cfg)
	assert.NoError(t, err)
	assert.NoError(t, AutoMigrate(db))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return db
}

func TestValidRoleSelection(t *testing.T) {
	svc := &AdminService{}
	admin := &models.User{Role: "admin"}
	manager := &models.User{Role: "manager"}

	// admin can assign any role
	ok, err := svc.ValidRoleSelection(admin, map[string]any{"role": "admin"})
	assert.True(t, ok)
	assert.Empty(t, err)

	// manager cannot assign admin role
	ok, err = svc.ValidRoleSelection(manager, map[string]any{"role": "admin"})
	assert.False(t, ok)
	assert.Equal(t, "Invalid role selection for user.", err)

	// manager can assign default/manager roles
	ok, _ = svc.ValidRoleSelection(manager, map[string]any{"role": "default"})
	assert.True(t, ok)
	ok, _ = svc.ValidRoleSelection(manager, map[string]any{"role": "manager"})
	assert.True(t, ok)

	// no role key in updates -> always valid
	ok, _ = svc.ValidRoleSelection(manager, map[string]any{"username": "x"})
	assert.True(t, ok)
}

func TestValidCanModify(t *testing.T) {
	svc := &AdminService{}
	admin := &models.User{Role: "admin"}
	manager := &models.User{Role: "manager"}
	defaultUser := &models.User{Role: "default"}
	otherAdmin := &models.User{Role: "admin"}

	// admin can modify anyone
	ok, _ := svc.ValidCanModify(admin, otherAdmin)
	assert.True(t, ok)

	// manager cannot modify admin
	ok, err := svc.ValidCanModify(manager, otherAdmin)
	assert.False(t, ok)
	assert.Equal(t, "Cannot perform that action on user.", err)

	// manager can modify default/manager
	ok, _ = svc.ValidCanModify(manager, defaultUser)
	assert.True(t, ok)
}

func TestAdminService_CreateUser(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewAdminService(db)

	u, errStr, err := svc.CreateUser(t.Context(), CreateUserInput{
		Username: "newbie",
		Password: "Password123!",
		Role:     "default",
	})
	assert.NoError(t, err)
	assert.Empty(t, errStr)
	assert.NotNil(t, u)
	assert.Equal(t, "newbie", *u.Username)
	assert.NotEqual(t, "Password123!", u.Password, "password must be hashed")

	// duplicate username -> business error returned (not Go error)
	_, errStr2, err2 := svc.CreateUser(t.Context(), CreateUserInput{
		Username: "newbie", Password: "Password123!", Role: "default",
	})
	assert.NoError(t, err2)
	assert.NotEmpty(t, errStr2)
}

func TestAdminService_UpdateUser(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewAdminService(db)

	created, _, _ := svc.CreateUser(t.Context(), CreateUserInput{
		Username: "u1", Password: "Password123!", Role: "default",
	})

	errStr, err := svc.UpdateUser(t.Context(), created.ID, map[string]any{
		"role": "manager",
	})
	assert.NoError(t, err)
	assert.Empty(t, errStr)

	got, err := svc.GetUserByID(t.Context(), created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "manager", got.Role)
}

func TestCanModifyAdmin_lastAdminLockout(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	_ = enc
	svc := NewAdminService(db)

	hash, _ := utils.HashPassword("pw")
	soleAdmin := &models.User{Username: utils.Ptr("solo"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(soleAdmin).Error)

	// trying to demote the only admin -> blocked
	ok, err := svc.CanModifyAdmin(soleAdmin, map[string]any{"role": "default"})
	assert.False(t, ok)
	assert.Contains(t, err, "No system admins")

	// adding a 2nd admin then demoting first is allowed
	otherAdmin := &models.User{Username: utils.Ptr("co"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(otherAdmin).Error)
	ok, _ = svc.CanModifyAdmin(soleAdmin, map[string]any{"role": "default"})
	assert.True(t, ok)
}
