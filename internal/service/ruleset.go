package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"diploma/internal/models"
	"diploma/internal/repository"
)

var (
	ErrRuleSetNotFound = errors.New("rule set not found")
	ErrRuleNotFound    = errors.New("rule not found")
	ErrInvalidRuleType = errors.New("invalid rule type")
)

type ruleSetService struct {
	repo repository.RuleSetRepository
}

func NewRuleSetService(repo repository.RuleSetRepository) RuleSetService {
	return &ruleSetService{repo: repo}
}

func (s *ruleSetService) CreateSet(ctx context.Context, req models.CreateRuleSetRequest, createdBy int64) (*models.RuleSet, error) {
	rs := &models.RuleSet{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
		CreatedBy:   createdBy,
	}
	if req.IsActive != nil {
		rs.IsActive = *req.IsActive
	}
	if err := s.repo.CreateSet(ctx, rs); err != nil {
		return nil, fmt.Errorf("ruleSetService.CreateSet: %w", err)
	}
	return rs, nil
}

func (s *ruleSetService) GetSet(ctx context.Context, id int64) (*models.RuleSetWithRules, error) {
	rs, err := s.repo.GetSetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("ruleSetService.GetSet: %w", err)
	}
	rules, err := s.repo.ListRulesBySet(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ruleSetService.GetSet: list rules: %w", err)
	}
	return &models.RuleSetWithRules{RuleSet: *rs, Rules: rules}, nil
}

func (s *ruleSetService) ListSets(ctx context.Context, activeOnly bool) ([]models.RuleSet, error) {
	return s.repo.ListSets(ctx, activeOnly)
}

func (s *ruleSetService) UpdateSet(ctx context.Context, id int64, req models.UpdateRuleSetRequest) (*models.RuleSet, error) {
	rs, err := s.repo.GetSetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("ruleSetService.UpdateSet: %w", err)
	}
	rs.Name = req.Name
	rs.Description = req.Description
	rs.IsActive = req.IsActive
	if err := s.repo.UpdateSet(ctx, rs); err != nil {
		return nil, fmt.Errorf("ruleSetService.UpdateSet: %w", err)
	}
	return rs, nil
}

func (s *ruleSetService) DeleteSet(ctx context.Context, id int64) error {
	if _, err := s.repo.GetSetByID(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRuleSetNotFound
		}
		return fmt.Errorf("ruleSetService.DeleteSet: %w", err)
	}
	return s.repo.DeleteSet(ctx, id)
}

func (s *ruleSetService) CreateRule(ctx context.Context, ruleSetID int64, req models.CreateRuleRequest) (*models.Rule, error) {
	if _, ok := models.AllowedRuleTypes[req.Type]; !ok {
		return nil, ErrInvalidRuleType
	}
	if _, err := s.repo.GetSetByID(ctx, ruleSetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("ruleSetService.CreateRule: %w", err)
	}
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return nil, fmt.Errorf("ruleSetService.CreateRule: marshal params: %w", err)
	}
	severity := req.Severity
	if severity == "" {
		severity = models.SeverityError
	}
	// Новое правило кладём в конец набора — порядок важен для section_order.
	existing, err := s.repo.ListRulesBySet(ctx, ruleSetID)
	if err != nil {
		return nil, fmt.Errorf("ruleSetService.CreateRule: list: %w", err)
	}
	pos := len(existing)
	rule := &models.Rule{
		RuleSetID: ruleSetID,
		Name:      req.Name,
		Type:      req.Type,
		Params:    models.JSONB(paramsBytes),
		Severity:  severity,
		Position:  pos,
	}
	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("ruleSetService.CreateRule: %w", err)
	}
	return rule, nil
}

func (s *ruleSetService) UpdateRule(ctx context.Context, ruleID int64, req models.UpdateRuleRequest) (*models.Rule, error) {
	if _, ok := models.AllowedRuleTypes[req.Type]; !ok {
		return nil, ErrInvalidRuleType
	}
	rule, err := s.repo.GetRuleByID(ctx, ruleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRuleNotFound
		}
		return nil, fmt.Errorf("ruleSetService.UpdateRule: %w", err)
	}
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return nil, fmt.Errorf("ruleSetService.UpdateRule: marshal params: %w", err)
	}
	rule.Name = req.Name
	rule.Type = req.Type
	rule.Params = models.JSONB(paramsBytes)
	if req.Severity != "" {
		rule.Severity = req.Severity
	}
	rule.Position = req.Position
	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("ruleSetService.UpdateRule: %w", err)
	}
	return rule, nil
}

func (s *ruleSetService) DeleteRule(ctx context.Context, ruleID int64) error {
	if _, err := s.repo.GetRuleByID(ctx, ruleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRuleNotFound
		}
		return fmt.Errorf("ruleSetService.DeleteRule: %w", err)
	}
	return s.repo.DeleteRule(ctx, ruleID)
}
