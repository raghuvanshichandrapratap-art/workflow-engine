package repos

import (
	"workflow-engine/models"

	"gorm.io/gorm"
)

type ExecutionLogRepository struct {
	db *gorm.DB
}

func NewExecutionLogRepository(db *gorm.DB) *ExecutionLogRepository {
	return &ExecutionLogRepository{db: db}
}

func (r *ExecutionLogRepository) Create(log *models.ExecutionLog) error {
	return r.db.Create(log).Error
}

func (r *ExecutionLogRepository) GetByWorkflowID(workflowID string) ([]models.ExecutionLog, error) {
	var logs []models.ExecutionLog
	if err := r.db.Where("workflow_id = ?", workflowID).Order("created_at desc").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}