package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"workflow-engine/logger"
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

	result, err := GenerateValidatedDSL(s.ai, workflow.Description)
	if err != nil {
		return nil, err
	}

	if !result.FinalValid {
		if result.RetryAttempted {
			return nil, fmt.Errorf("AI failed to generate a valid workflow after retry: %s", joinErrors(result.ValidationErrors))
		}
		return nil, fmt.Errorf("AI failed to generate a valid workflow: %s", joinErrors(result.ValidationErrors))
	}

	return s.saveDSL(workflow, result.DSL)
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

func (s *WorkflowService) ExecuteWorkflow(id string, input map[string]interface{}) (result *models.ExecutionResult, err error) {
	executionID := logger.NewID()
	start := time.Now()
	logger.Metrics.ExecutionsStarted.Add(1)
	slog.Info("execution started", "execution_id", executionID, "workflow_id", id)

	defer func() {
		duration := time.Since(start).Milliseconds()
		if err != nil {
			logger.Metrics.ExecutionsFailed.Add(1)
			slog.Error("execution failed", "execution_id", executionID, "workflow_id", id, "duration_ms", duration, "error", err.Error())
			return
		}
		logger.Metrics.ExecutionsSucceeded.Add(1)
		slog.Info("execution completed", "execution_id", executionID, "workflow_id", id, "duration_ms", duration, "status", result.Status)
	}()

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

	result, err = s.engine.Execute(&dsl, input)
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
