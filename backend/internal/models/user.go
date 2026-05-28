package models

import "time"

type User struct {
	ID                        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Username                  *string   `gorm:"unique" json:"username"`
	Password                  string    `json:"-"`
	PfpFilename               *string   `json:"pfpFilename"`
	Role                      string    `gorm:"default:default" json:"role"`
	Suspended                 int       `gorm:"default:0" json:"suspended"`
	SeenRecoveryCodes         *bool     `gorm:"default:false" json:"seenRecoveryCodes"`
	CreatedAt                 time.Time `json:"createdAt"`
	LastUpdatedAt             time.Time `json:"lastUpdatedAt"`
	DailyMessageLimit         *int      `json:"dailyMessageLimit"`
	Bio                       *string   `json:"bio"`
	WebPushSubscriptionConfig *string   `json:"webPushSubscriptionConfig"`
}
