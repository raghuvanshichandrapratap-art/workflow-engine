package models

import (
	"encoding/json"
	"time"
)

type ExecutionLog struct {
	ID         uint            `gorm:"primaryKey" json:"id"`
	WorkflowID uint            `json:"workflow_id"`
	Status     string          `json:"status"`
	Input      json.RawMessage `gorm:"type:jsonb" json:"input"`
	Steps      json.RawMessage `gorm:"type:jsonb" json:"steps"`
	CreatedAt  time.Time       `json:"created_at"`
}