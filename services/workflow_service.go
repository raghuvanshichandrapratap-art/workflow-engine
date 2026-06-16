package services

import (
	"workflow-engine/models"
	"workflow-engine/repos"
)

type WorkflowService struct {
	repo *repos.WorkflowRepository
}

func NewWorkflowService(repo *repos.WorkflowRepository) *WorkflowService {
	return &WorkflowService{repo: repo}
}

func (s *WorkflowService) CreateWorkflow(name string, description string) (*models.Workflow, error) {

	workflow := &models.Workflow{
		Name:        name,
		Description: description,
	}

	if err := s.repo.Create(workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}
