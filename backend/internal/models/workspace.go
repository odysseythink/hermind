package models

import "time"

type Workspace struct {
	ID                   int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                 string    `json:"name"`
	Slug                 string    `gorm:"unique" json:"slug"`
	VectorTag            *string   `json:"vectorTag"`
	CreatedAt            time.Time `json:"createdAt"`
	OpenAiTemp           *float64  `json:"openAiTemp"`
	OpenAiHistory        int       `gorm:"default:20" json:"openAiHistory"`
	LastUpdatedAt        time.Time `json:"lastUpdatedAt"`
	OpenAiPrompt         *string   `json:"openAiPrompt"`
	SimilarityThreshold  *float64  `gorm:"default:0.25" json:"similarityThreshold"`
	ChatProvider         *string   `json:"chatProvider"`
	ChatModel            *string   `json:"chatModel"`
	TopN                 *int      `gorm:"default:4" json:"topN"`
	ChatMode             *string   `gorm:"default:chat" json:"chatMode"`
	PfpFilename          *string   `json:"pfpFilename"`
	AgentProvider        *string   `json:"agentProvider"`
	AgentModel           *string   `json:"agentModel"`
	QueryRefusalResponse *string   `json:"queryRefusalResponse"`
	VectorSearchMode     *string   `gorm:"default:default" json:"vectorSearchMode"`
}
