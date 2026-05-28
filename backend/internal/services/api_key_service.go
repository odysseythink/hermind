package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type APIKeyService struct {
	db *gorm.DB
}

func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

func (s *APIKeyService) List(ctx context.Context) ([]models.APIKey, error) {
	var keys []models.APIKey
	err := s.db.WithContext(ctx).Order("id desc").Find(&keys).Error
	return keys, err
}

func (s *APIKeyService) Create(ctx context.Context, createdBy *int, name *string) (*models.APIKey, error) {
	secret := uuid.New().String()
	key := models.APIKey{
		Secret:        &secret,
		CreatedBy:     createdBy,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if name != nil && *name != "" {
		key.Name = name
	}
	if err := s.db.WithContext(ctx).Create(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *APIKeyService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.APIKey{}, id).Error
}

type APIKeyWithUser struct {
	ID            int       `json:"id"`
	Name          *string   `json:"name"`
	Secret        *string   `json:"secret"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
	CreatedBy     any       `json:"createdBy"` // nil OR {id,username,role}
}

func (s *APIKeyService) ListWithUser(ctx context.Context) ([]APIKeyWithUser, error) {
	var keys []models.APIKey
	if err := s.db.WithContext(ctx).Order("id desc").Find(&keys).Error; err != nil {
		return nil, err
	}
	uids := []int{}
	for _, k := range keys {
		if k.CreatedBy != nil {
			uids = append(uids, *k.CreatedBy)
		}
	}
	var users []models.User
	if len(uids) > 0 {
		if err := s.db.WithContext(ctx).Where("id IN ?", uids).Find(&users).Error; err != nil {
			return nil, err
		}
	}
	byID := map[int]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	out := make([]APIKeyWithUser, 0, len(keys))
	for _, k := range keys {
		entry := APIKeyWithUser{
			ID: k.ID, Name: k.Name, Secret: k.Secret,
			CreatedAt: k.CreatedAt, LastUpdatedAt: k.LastUpdatedAt,
		}
		if k.CreatedBy != nil {
			if u, ok := byID[*k.CreatedBy]; ok {
				username := ""
				if u.Username != nil {
					username = *u.Username
				}
				entry.CreatedBy = map[string]any{
					"id": u.ID, "username": username, "role": u.Role,
				}
			} else {
				entry.CreatedBy = *k.CreatedBy
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *APIKeyService) ValidateKey(ctx context.Context, secret string) (*models.APIKey, error) {
	var key models.APIKey
	if err := s.db.WithContext(ctx).Where("secret = ?", secret).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}
