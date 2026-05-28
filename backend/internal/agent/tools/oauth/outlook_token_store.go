package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

var ErrTokenNotFound = errors.New("outlook token not found")

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Tenant       string
}

type TokenStore struct {
	db  *gorm.DB
	enc *utils.EncryptionManager
}

func NewTokenStore(db *gorm.DB, enc *utils.EncryptionManager) *TokenStore {
	return &TokenStore{db: db, enc: enc}
}

func (s *TokenStore) Get(ctx context.Context, userID int) (*TokenSet, error) {
	var row models.OutlookOAuthToken
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}

	at, err := s.enc.Decrypt(row.EncryptedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt access: %w", err)
	}
	rt, err := s.enc.Decrypt(row.EncryptedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh: %w", err)
	}

	return &TokenSet{
		AccessToken: at, RefreshToken: rt,
		ExpiresAt: row.ExpiresAt, Tenant: row.Tenant,
	}, nil
}

func (s *TokenStore) Save(ctx context.Context, userID int, ts *TokenSet) error {
	at, err := s.enc.Encrypt(ts.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt access: %w", err)
	}
	rt, err := s.enc.Encrypt(ts.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh: %w", err)
	}

	row := models.OutlookOAuthToken{
		UserID: userID, Tenant: ts.Tenant,
		EncryptedAccessToken: at, EncryptedRefreshToken: rt,
		ExpiresAt: ts.ExpiresAt,
	}
	// Upsert by user_id
	return s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Assign(row).
		FirstOrCreate(&row).Error
}

func (s *TokenStore) Delete(ctx context.Context, userID int) error {
	return s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.OutlookOAuthToken{}).Error
}

// DB returns the underlying gorm.DB for transaction/locking use.
func (s *TokenStore) DB() *gorm.DB {
	return s.db
}
