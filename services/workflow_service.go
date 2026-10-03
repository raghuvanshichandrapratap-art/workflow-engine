package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"workflow-engine/models"
	"workflow-engine/repos"
)

type WorkflowService struct {
	repo    *repos.WorkflowRepository
	logRepo *repos.ExecutionLogRepository
	ai      *AIService
	engine  *ExecutionEngine
}

func NewWorkflowService(repo *repos.WorkflowRepository, logRepo *repos.ExecutionLogRepository, ai *AIService, engine *ExecutionEngine) *WorkflowService {
	return &WorkflowService{repo: repo, logRepo: logRepo, ai: ai, engine: engine}
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

	dsl, raw, err := s.ai.GenerateDSL(workflow.Description)

	var problems []error
	switch {
	case err != nil && raw == "":
		// The API call itself failed (network, rate limit, etc.) — nothing to
		// correct, so no retry on it.
		return nil, fmt.Errorf("AI request failed: %w", err)
	case err != nil:
		// AI replied, but it wasn't valid JSON at all.
		problems = []error{err}
	default:
		problems = ValidateDSL(dsl)
	}

	if len(problems) == 0 {
		return s.saveDSL(workflow, dsl)
	}

	retryDSL, _, retryErr := s.ai.RetryDSL(workflow.Description, raw, problems)
	if retryErr != nil {
		return nil, fmt.Errorf("AI failed to generate a valid workflow: %w", retryErr)
	}

	if retryProblems := ValidateDSL(retryDSL); len(retryProblems) > 0 {
		return nil, fmt.Errorf("AI failed to generate a valid workflow after retry: %s", joinErrors(retryProblems))
	}

	return s.saveDSL(workflow, retryDSL)
}

func (s *WorkflowService) saveDSL(workflow *models.Workflow, dsl *models.DSL) (*models.Workflow, error) {
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

func joinErrors(errs []error) string {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
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

	inputBytes, _ := json.Marshal(input)
	stepsBytes, _ := json.Marshal(result.Steps)
	s.logRepo.Create(&models.ExecutionLog{
		WorkflowID: workflow.ID,
		Status:     result.Status,
		Input:      json.RawMessage(inputBytes),
		Steps:      json.RawMessage(stepsBytes),
	})

	return result, nil
}

func (s *WorkflowService) GetExecutionHistory(workflowID string, page, limit int) ([]models.ExecutionLog, int64, error) {
	offset := (page - 1) * limit
	return s.logRepo.GetByWorkflowID(workflowID, offset, limit)
}

func (s *WorkflowService) DeleteWorkflow(id string) error {
	workflow, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	return s.repo.DeleteByID(workflow.ID)
}
