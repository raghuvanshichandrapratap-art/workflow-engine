package repos

import (
	"workflow-engine/models"

	"gorm.io/gorm"
)

type WorkflowRepository struct {
	db *gorm.DB
}

func NewWorkflowRepository(db *gorm.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) Create(workflow *models.Workflow) error {
	return r.db.Create(workflow).Error
}

func (r *WorkflowRepository) GetByID(id string) (*models.Workflow, error) {
	var workflow models.Workflow
	if err := r.db.First(&workflow, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &workflow, nil
}

func (r *WorkflowRepository) Update(workflow *models.Workflow) error {
	return r.db.Save(workflow).Error
}

func (r *WorkflowRepository) DeleteByID(id uint) error {
	return r.db.Delete(&models.Workflow{}, id).Error
}
