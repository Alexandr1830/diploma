package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"diploma/internal/compliance"
	"diploma/internal/models"
	"diploma/internal/repository"
)

var ErrEmptyParsedText = errors.New("version has no parsed text — cannot run compliance check")

type complianceService struct {
	docs   repository.DocumentRepository
	vers   repository.DocumentVersionRepository
	sets   repository.RuleSetRepository
	checks repository.ComplianceCheckRepository
	engine *compliance.Engine
}

// NewComplianceService собирает все зависимости, нужные для прогона
// проверки: документ (для проверки прав через canRead), версия (для
// parsed_text), набор правил и место, куда писать результат.
func NewComplianceService(
	docs repository.DocumentRepository,
	vers repository.DocumentVersionRepository,
	sets repository.RuleSetRepository,
	checks repository.ComplianceCheckRepository,
) ComplianceService {
	return &complianceService{
		docs:   docs,
		vers:   vers,
		sets:   sets,
		checks: checks,
		engine: compliance.New(),
	}
}

// Run прогоняет все правила ruleSetID по parsed_text версии, сохраняет
// результат в compliance_checks и возвращает его.
//
// Доступ: writer-владелец, ревьюер или admin. Developer'у проверки не даём
// даже на опубликованных документах — compliance это часть подготовки
// документа, после публикации она уже не имеет смысла.
func (s *complianceService) Run(ctx context.Context, docID, versionID, ruleSetID, userID int64, role string) (*models.ComplianceCheck, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("complianceService.Run: get doc: %w", err)
	}
	if !canReadInternal(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("complianceService.Run: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}
	if !v.ParsedText.Valid || v.ParsedText.String == "" {
		return nil, ErrEmptyParsedText
	}

	rs, err := s.sets.GetSetByID(ctx, ruleSetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("complianceService.Run: get set: %w", err)
	}
	rules, err := s.sets.ListRulesBySet(ctx, rs.ID)
	if err != nil {
		return nil, fmt.Errorf("complianceService.Run: list rules: %w", err)
	}

	results := s.engine.EvaluateAll(rules, v.ParsedText.String)
	passed, failed := 0, 0
	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("complianceService.Run: marshal results: %w", err)
	}
	check := &models.ComplianceCheck{
		VersionID:   versionID,
		RuleSetID:   ruleSetID,
		TotalRules:  len(rules),
		PassedRules: passed,
		FailedRules: failed,
		Results:     models.JSONB(resultsJSON),
		CreatedBy:   userID,
	}
	if err := s.checks.Create(ctx, check); err != nil {
		return nil, fmt.Errorf("complianceService.Run: persist: %w", err)
	}
	return check, nil
}

func (s *complianceService) ListByVersion(ctx context.Context, docID, versionID, userID int64, role string) ([]models.ComplianceCheck, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("complianceService.ListByVersion: get doc: %w", err)
	}
	if !canReadInternal(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}
	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("complianceService.ListByVersion: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}
	return s.checks.ListByVersion(ctx, versionID)
}

// canReadInternal — то же что canRead, но без послабления «developer видит
// опубликованное». Compliance — внутренняя кухня, библиотечному читателю
// она не нужна.
func canReadInternal(doc *models.Document, userID int64, role models.UserRole) bool {
	switch role {
	case models.RoleAdmin:
		return true
	case models.RoleWriter:
		return doc.CreatedBy == userID
	case models.RoleReviewer:
		return doc.ReviewerID.Valid && doc.ReviewerID.Int64 == userID
	default:
		return false
	}
}
