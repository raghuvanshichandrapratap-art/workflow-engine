package services

import (
	"encoding/json"
	"fmt"
	"workflow-engine/models"
	"workflow-engine/repos"
)

type WorkflowService struct {
	repo   *repos.WorkflowRepository
	ai     *AIService
	engine *ExecutionEngine
}

func NewWorkflowService(repo *repos.WorkflowRepository, ai *AIService, engine *ExecutionEngine) *WorkflowService {
	return &WorkflowService{repo: repo, ai: ai, engine: engine}
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

	dslBytes, err := json.Marshal(dsl)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize DSL: %w", err)
	}

	workflow.DSL = json.RawMessage(dslBytes)

	if err := s.repo.Update(workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (s *WorkflowService) ExecuteWorkflow(id string, input map[string]interface{}) (*models.ExecutionResult, error) {
	workflow, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if workflow.DSL == nil {
		return nil, fmt.Errorf("workflow has no DSL, generate it first")
	}

	var dsl models.DSL
	if err := json.Unmarshal(workflow.DSL, &dsl); err != nil {
		return nil, fmt.Errorf("failed to parse DSL: %w", err)
	}

	result, err := s.engine.Execute(&dsl, input)
	if err != nil {
		return nil, err
	}

	workflow.Status = result.Status
	if err := s.repo.Update(workflow); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *WorkflowService) DeleteWorkflow(id string) error {
	workflow, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	return s.repo.DeleteByID(workflow.ID)
}
