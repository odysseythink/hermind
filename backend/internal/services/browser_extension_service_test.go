package services

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestBrowserExtensionService(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewBrowserExtensionService(db)
	ctx := t.Context()

	uid1 := 1
	uid2 := 2

	t.Run("CreateKey", func(t *testing.T) {
		key, err := svc.CreateKey(ctx, &uid1)
		assert.NoError(t, err)
		assert.NotNil(t, key)
		assert.NotZero(t, key.ID)
		assert.True(t, len(key.Key) > 4)
		assert.Equal(t, &uid1, key.UserID)
	})

	t.Run("ListKeys admin sees all", func(t *testing.T) {
		// clean slate
		db.Where("1 = 1").Delete(&models.BrowserExtensionApiKey{})

		_, err := svc.CreateKey(ctx, &uid1)
		assert.NoError(t, err)
		_, err = svc.CreateKey(ctx, &uid2)
		assert.NoError(t, err)

		keys, err := svc.ListKeys(ctx, &uid1, true)
		assert.NoError(t, err)
		assert.Len(t, keys, 2)
	})

	t.Run("ListKeys non-admin sees own", func(t *testing.T) {
		keys, err := svc.ListKeys(ctx, &uid1, false)
		assert.NoError(t, err)
		for _, k := range keys {
			assert.NotNil(t, k.UserID)
			assert.Equal(t, uid1, *k.UserID)
		}
	})

	t.Run("DeleteKey admin can delete any", func(t *testing.T) {
		db.Where("1 = 1").Delete(&models.BrowserExtensionApiKey{})

		key, err := svc.CreateKey(ctx, &uid2)
		assert.NoError(t, err)

		err = svc.DeleteKey(ctx, key.ID, &uid1, true)
		assert.NoError(t, err)

		var count int64
		db.Model(&models.BrowserExtensionApiKey{}).Count(&count)
		assert.Zero(t, count)
	})

	t.Run("DeleteKey non-admin can delete own", func(t *testing.T) {
		key, err := svc.CreateKey(ctx, &uid1)
		assert.NoError(t, err)

		err = svc.DeleteKey(ctx, key.ID, &uid1, false)
		assert.NoError(t, err)
	})

	t.Run("DeleteKey non-admin cannot delete others", func(t *testing.T) {
		key, err := svc.CreateKey(ctx, &uid2)
		assert.NoError(t, err)

		err = svc.DeleteKey(ctx, key.ID, &uid1, false)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("Validate valid key", func(t *testing.T) {
		key, err := svc.CreateKey(ctx, &uid1)
		assert.NoError(t, err)

		found, err := svc.Validate(ctx, key.Key)
		assert.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, key.ID, found.ID)
	})

	t.Run("Validate invalid prefix", func(t *testing.T) {
		found, err := svc.Validate(ctx, "invalid-key")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
		assert.Nil(t, found)
	})

	t.Run("Validate non-existent key", func(t *testing.T) {
		found, err := svc.Validate(ctx, "brx-00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
		assert.Nil(t, found)
	})

	t.Run("DeleteAllForUser", func(t *testing.T) {
		db.Where("1 = 1").Delete(&models.BrowserExtensionApiKey{})

		_, err := svc.CreateKey(ctx, &uid1)
		assert.NoError(t, err)
		_, err = svc.CreateKey(ctx, &uid1)
		assert.NoError(t, err)
		_, err = svc.CreateKey(ctx, &uid2)
		assert.NoError(t, err)

		err = svc.DeleteAllForUser(ctx, uid1)
		assert.NoError(t, err)

		keys, err := svc.ListKeys(ctx, nil, true)
		assert.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Equal(t, &uid2, keys[0].UserID)
	})
}
