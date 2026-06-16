package models

type Workflow struct {
	ID          uint `gorm:"primaryKey"`
	Name        string
	Description string
	DSL         string
	Status      string
}
