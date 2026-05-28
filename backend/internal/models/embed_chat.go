package models

import "time"

type EmbedChat struct {
	ID                    int         `gorm:"primaryKey;autoIncrement" json:"id"`
	Prompt                string      `json:"prompt"`
	Response              string      `json:"response"`
	SessionID             string      `json:"sessionId"`
	Include               bool        `gorm:"default:true" json:"include"`
	ConnectionInformation *string     `json:"connectionInformation"`
	EmbedID               int         `json:"embedId"`
	EmbedConfig           EmbedConfig `gorm:"foreignKey:EmbedID" json:"embedConfig,omitempty"`
	UserID                *int        `json:"userId"`
	CreatedAt             time.Time   `json:"createdAt"`
}
