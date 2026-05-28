package models

import "time"

type DocumentVector struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	DocId         string    `json:"docId"`
	VectorId      string    `json:"vectorId"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
