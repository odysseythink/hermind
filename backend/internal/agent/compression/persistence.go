package compression

import (
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

// CompactionStore persists and loads ThreadCompaction records.
type CompactionStore struct {
	db *gorm.DB
}

// NewCompactionStore creates a new store backed by the given GORM DB.
func NewCompactionStore(db *gorm.DB) *CompactionStore {
	return &CompactionStore{db: db}
}

// LoadLatest returns the most recent ThreadCompaction for a given workspace
// and optional thread. If threadID is nil, it matches rows where thread_id IS NULL.
// Returns nil if no compaction exists.
func (s *CompactionStore) LoadLatest(workspaceID int, threadID *int) (*models.ThreadCompaction, error) {
	var c models.ThreadCompaction
	q := s.db.Where("workspace_id = ?", workspaceID)
	if threadID != nil {
		q = q.Where("thread_id = ?", *threadID)
	} else {
		q = q.Where("thread_id IS NULL")
	}
	if err := q.Order("created_at DESC").First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("load latest compaction: %w", err)
	}
	return &c, nil
}

// Save inserts a new ThreadCompaction record.
func (s *CompactionStore) Save(c *models.ThreadCompaction) error {
	if err := s.db.Create(c).Error; err != nil {
		return fmt.Errorf("save compaction: %w", err)
	}
	return nil
}

// SeedForSession returns the latest summary and UpToChatID for a workspace/thread
// pair, or empty values if none exists. This is used to initialize a compressor
// with its previous summary at session start.
func (s *CompactionStore) SeedForSession(workspaceID int, threadID *int) (summary string, upToChatID int, err error) {
	c, err := s.LoadLatest(workspaceID, threadID)
	if err != nil {
		return "", 0, err
	}
	if c == nil {
		return "", 0, nil
	}
	return c.Summary, c.UpToChatID, nil
}
