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

// // get a workflow by ID
// func (r *WorkflowRepository) GetByID(id uint) (*models.Workflow, error) {
// 	var workflow models.Workflow
// 	if err := r.db.First(&workflow, id).Error; err != nil {
// 		return nil, err
// 	}
// 	return &workflow, nil
// }

// // delete a workflow by ID
// func (r *WorkflowRepository) DeleteByID(id uint) error {
// 	return r.db.Delete(&models.Workflow{}, id).Error
// }
