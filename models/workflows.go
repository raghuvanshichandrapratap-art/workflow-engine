package models

import "encoding/json"

type Workflow struct {
	ID          uint            `gorm:"primaryKey"`
	Name        string
	Description string
	DSL         json.RawMessage `gorm:"type:jsonb;default:null"`
	Status      string
}
