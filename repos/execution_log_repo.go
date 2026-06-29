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

func (r *ExecutionLogRepository) GetByWorkflowID(workflowID string, offset, limit int) ([]models.ExecutionLog, int64, error) {
	var logs []models.ExecutionLog
	var total int64

	r.db.Model(&models.ExecutionLog{}).Where("workflow_id = ?", workflowID).Count(&total)

	if err := r.db.Where("workflow_id = ?", workflowID).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}