package compression

import (
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCompactionStore_LoadLatest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&models.ThreadCompaction{}))

	store := NewCompactionStore(db)

	// No compaction yet
	c, err := store.LoadLatest(1, intPtr(2))
	require.NoError(t, err)
	assert.Nil(t, c)

	// Create two compactions for the same workspace+thread
	require.NoError(t, db.Create(&models.ThreadCompaction{
		WorkspaceID: 1, ThreadID: intPtr(2), Summary: "old", UpToChatID: 5,
		CreatedAt: time.Now().Add(-time.Hour), LastUpdatedAt: time.Now().Add(-time.Hour),
	}).Error)
	require.NoError(t, db.Create(&models.ThreadCompaction{
		WorkspaceID: 1, ThreadID: intPtr(2), Summary: "new", UpToChatID: 10,
		CreatedAt: time.Now(), LastUpdatedAt: time.Now(),
	}).Error)

	latest, err := store.LoadLatest(1, intPtr(2))
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "new", latest.Summary)
	assert.Equal(t, 10, latest.UpToChatID)

	// Different thread should return nil
	other, err := store.LoadLatest(1, intPtr(99))
	require.NoError(t, err)
	assert.Nil(t, other)
}

func TestCompactionStore_LoadLatest_NilThread(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&models.ThreadCompaction{}))

	store := NewCompactionStore(db)

	require.NoError(t, db.Create(&models.ThreadCompaction{
		WorkspaceID: 1, ThreadID: nil, Summary: "default session", UpToChatID: 3,
		CreatedAt: time.Now(), LastUpdatedAt: time.Now(),
	}).Error)

	latest, err := store.LoadLatest(1, nil)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "default session", latest.Summary)
}

func TestCompactionStore_Save(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&models.ThreadCompaction{}))

	store := NewCompactionStore(db)

	c := &models.ThreadCompaction{
		WorkspaceID: 1,
		ThreadID:    intPtr(2),
		Summary:     "saved summary",
		UpToChatID:  7,
	}
	require.NoError(t, store.Save(c))
	assert.NotZero(t, c.ID)

	var loaded models.ThreadCompaction
	require.NoError(t, db.First(&loaded, c.ID).Error)
	assert.Equal(t, "saved summary", loaded.Summary)
	assert.Equal(t, 7, loaded.UpToChatID)
}

func TestCompactionStore_SeedForSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&models.ThreadCompaction{}))

	store := NewCompactionStore(db)

	// Seed with an existing compaction
	require.NoError(t, db.Create(&models.ThreadCompaction{
		WorkspaceID: 1, ThreadID: intPtr(2), Summary: "seed summary", UpToChatID: 4,
		CreatedAt: time.Now(), LastUpdatedAt: time.Now(),
	}).Error)

	summary, upToID, err := store.SeedForSession(1, intPtr(2))
	require.NoError(t, err)
	assert.Equal(t, "seed summary", summary)
	assert.Equal(t, 4, upToID)

	// Seed with no compaction
	summary, upToID, err = store.SeedForSession(1, intPtr(99))
	require.NoError(t, err)
	assert.Equal(t, "", summary)
	assert.Equal(t, 0, upToID)
}

func intPtr(i int) *int { return &i }
