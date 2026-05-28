package models

import "time"

type OutlookOAuthToken struct {
	ID                    int       `gorm:"primaryKey"`
	UserID                int       `gorm:"uniqueIndex;not null"`
	Tenant                string    `gorm:"not null"`
	EncryptedAccessToken  string    `gorm:"type:text;not null"`
	EncryptedRefreshToken string    `gorm:"type:text;not null"`
	ExpiresAt             time.Time `gorm:"not null"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (OutlookOAuthToken) TableName() string { return "outlook_oauth_tokens" }
