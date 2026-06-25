package services

import (
	"workflow-engine/models"
	"workflow-engine/repos"
)

type WorkflowService struct {
	repo *repos.WorkflowRepository
	ai   *AIService
}

func NewWorkflowService(repo *repos.WorkflowRepository, ai *AIService) *WorkflowService {
	return &WorkflowService{repo: repo, ai: ai}
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

func (s *WorkflowService) GetWorkflow(id string) (*models.Workflow, error) {
	return s.repo.GetByID(id)
}

func (s *WorkflowService) UpdateWorkflow(id string, name string, description string) (*models.Workflow, error) {
	workflow, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	workflow.Name = name
	workflow.Description = description

	if err := s.repo.Update(workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (s *WorkflowService) GenerateDSL(id string) (*models.Workflow, error) {
	workflow, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	dsl, err := s.ai.GenerateDSL(workflow.Description)
	if err != nil {
		return nil, err
	}

	workflow.DSL = dsl

	if err := s.repo.Update(workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (s *WorkflowService) DeleteWorkflow(id string) error {
	workflow, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	return s.repo.DeleteByID(workflow.ID)
}
