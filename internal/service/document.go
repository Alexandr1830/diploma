package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"diploma/internal/models"
	"diploma/internal/repository"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrForbidden        = errors.New("forbidden")
	ErrNotDraft         = errors.New("document must be in draft status")
	ErrTitleTaken       = errors.New("document title already used")
	ErrNotPublished     = errors.New("document is not published")
)

type documentService struct {
	docs repository.DocumentRepository
}

func NewDocumentService(docs repository.DocumentRepository) DocumentService {
	return &documentService{docs: docs}
}

// Create creates a new document on behalf of userID.
// Status is always draft; version fields are left empty.
//
// Если caller — admin или reviewer и в запросе передан WriterID, документ
// создаётся от имени этого writer'а (created_by = WriterID). Это позволяет
// админам и ревьюерам заводить документы для writer'ов. Для writer/developer
// поле игнорируется — caller всегда становится владельцем.
func (s *documentService) Create(ctx context.Context, req models.CreateDocumentRequest, userID int64, role string) (*models.Document, error) {
	// Уникальность названия (case-insensitive) — иначе пользователю будет
	// сложно различать документы в списке. ErrTitleTaken мапится в 409.
	if existing, err := s.docs.GetByTitle(ctx, req.Title); err == nil && existing != nil {
		return nil, ErrTitleTaken
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("documentService.Create: title check: %w", err)
	}

	createdBy := userID
	r := models.UserRole(role)
	if (r == models.RoleAdmin || r == models.RoleReviewer) && req.WriterID != nil && *req.WriterID > 0 {
		createdBy = *req.WriterID
	}
	doc := &models.Document{
		Title:       req.Title,
		Description: req.Description,
		ProjectID:   req.ProjectID,
		CategoryID:  req.CategoryID,
		CreatedBy:   createdBy,
		Status:      models.StatusDraft,
	}
	if req.ReviewerID != nil {
		doc.ReviewerID = sql.NullInt64{Int64: *req.ReviewerID, Valid: true}
	}
	if err := s.docs.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("documentService.Create: %w", err)
	}
	return doc, nil
}

// List returns documents visible to the caller based on their role.
//
//   writer    — собственные документы (любой статус, включая draft)
//   reviewer  — назначенные на него
//   admin     — всё
//   остальные — пусто
//
// Дополнительно: черновик (status=draft) виден ТОЛЬКО его автору
// (created_by). Даже admin не видит чужой draft в списке.
func (s *documentService) List(ctx context.Context, userID int64, role string, q models.DocumentQuery) ([]models.Document, error) {
	f := repository.DocumentFilters{
		ProjectID:  q.ProjectID,
		CategoryID: q.CategoryID,
		Status:     q.Status,
	}
	r := models.UserRole(role)
	switch r {
	case models.RoleWriter:
		f.CreatedBy = &userID
	case models.RoleReviewer:
		f.ReviewerID = &userID
	case models.RoleAdmin:
		// no additional filter — sees everything
	default:
		// developer and unknown roles: no working documents
		return []models.Document{}, nil
	}
	docs, err := s.docs.List(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("documentService.List: %w", err)
	}
	// Скрываем чужие черновики ото всех. Свои черновики автор видит — для
	// него created_by совпадает с userID. Применяется ко всем ролям, включая
	// admin: он не должен видеть, что writer ещё в процессе наброска.
	filtered := docs[:0]
	for _, d := range docs {
		if d.Status == models.StatusDraft && d.CreatedBy != userID {
			continue
		}
		filtered = append(filtered, d)
	}
	docs = filtered
	return docs, nil
}

// GetByID fetches one document and checks that the caller is allowed to read it.
func (s *documentService) GetByID(ctx context.Context, id, userID int64, role string) (*models.Document, error) {
	doc, err := s.docs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("documentService.GetByID: %w", err)
	}
	if !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}
	return doc, nil
}

// Update меняет метаданные документа (title/description/category/reviewer).
// Доступ: writer-владелец, reviewer-назначенный или admin (см. canEditMeta).
func (s *documentService) Update(ctx context.Context, id int64, req models.UpdateDocumentRequest, userID int64, role string) (*models.Document, error) {
	doc, err := s.docs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("documentService.Update: fetch: %w", err)
	}
	if !canEditMeta(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	// Уникальность названия при переименовании. Если новое название
	// совпадает (без учёта регистра) с другим существующим документом —
	// ErrTitleTaken. Тот же title (даже если меняется регистр) у самого
	// себя ошибки не вызывает.
	if req.Title != doc.Title {
		if existing, err := s.docs.GetByTitle(ctx, req.Title); err == nil && existing != nil && existing.ID != id {
			return nil, ErrTitleTaken
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("documentService.Update: title check: %w", err)
		}
	}

	doc.Title = req.Title
	doc.Description = req.Description
	doc.CategoryID = req.CategoryID
	if req.ReviewerID != nil {
		doc.ReviewerID = sql.NullInt64{Int64: *req.ReviewerID, Valid: true}
	} else {
		doc.ReviewerID = sql.NullInt64{} // null → unset reviewer
	}

	if err := s.docs.Update(ctx, doc); err != nil {
		return nil, fmt.Errorf("documentService.Update: save: %w", err)
	}
	return doc, nil
}

// Delete removes a document.
// writer can only delete their own draft; admin can delete anything.
func (s *documentService) Delete(ctx context.Context, id, userID int64, role string) error {
	doc, err := s.docs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDocumentNotFound
		}
		return fmt.Errorf("documentService.Delete: fetch: %w", err)
	}

	switch models.UserRole(role) {
	case models.RoleAdmin:
		// admin bypasses all restrictions
	case models.RoleWriter:
		if doc.CreatedBy != userID {
			return ErrForbidden
		}
		if doc.Status != models.StatusDraft {
			return ErrNotDraft
		}
	default:
		return ErrForbidden
	}

	if err := s.docs.Delete(ctx, id); err != nil {
		return fmt.Errorf("documentService.Delete: %w", err)
	}
	return nil
}

// Publish transitions approved → published.
// Only writer (owner) or admin may publish. Requires current_version_id.
func (s *documentService) Publish(ctx context.Context, id, userID int64, role string) (*models.Document, error) {
	doc, err := s.docs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("documentService.Publish: fetch: %w", err)
	}

	if !canWrite(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	if doc.Status != models.StatusApproved {
		return nil, fmt.Errorf("document must be in approved status: %w", ErrNotDraft)
	}

	if !doc.CurrentVersionID.Valid {
		return nil, fmt.Errorf("no current version: %w", ErrNotDraft)
	}

	if err := s.docs.SetPublishedVersion(ctx, id, doc.CurrentVersionID.Int64, userID); err != nil {
		return nil, fmt.Errorf("documentService.Publish: %w", err)
	}

	doc, err = s.docs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("documentService.Publish: re-fetch: %w", err)
	}
	return doc, nil
}

// Unpublish reverts a published document back to approved status.
// Allowed: writer-owner, assigned reviewer, or admin.
// The document's published_version_id and published_at remain intact —
// they are convenient breadcrumbs ("was last published on …") and an
// admin can re-publish quickly. The document just disappears from /library.
func (s *documentService) Unpublish(ctx context.Context, id, userID int64, role string) (*models.Document, error) {
	doc, err := s.docs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("documentService.Unpublish: fetch: %w", err)
	}

	if doc.Status != models.StatusPublished {
		return nil, ErrNotPublished
	}

	r := models.UserRole(role)
	allowed := r == models.RoleAdmin ||
		(r == models.RoleWriter && doc.CreatedBy == userID) ||
		(r == models.RoleReviewer && doc.ReviewerID.Valid && doc.ReviewerID.Int64 == userID)
	if !allowed {
		return nil, ErrForbidden
	}

	if err := s.docs.UpdateStatus(ctx, id, models.StatusApproved); err != nil {
		return nil, fmt.Errorf("documentService.Unpublish: %w", err)
	}
	doc.Status = models.StatusApproved
	return doc, nil
}

// ---------------------------------------------------------------------------
// access-control helpers
// ---------------------------------------------------------------------------

func canRead(doc *models.Document, userID int64, role models.UserRole) bool {
	// Черновик — приватный документ автора. Никто кроме created_by его не видит,
	// даже admin. Это соответствует ожиданию пользователя: документ становится
	// общим достоянием только после submit на ревью.
	if doc.Status == models.StatusDraft {
		return doc.CreatedBy == userID
	}
	switch role {
	case models.RoleAdmin:
		return true
	case models.RoleWriter:
		return doc.CreatedBy == userID
	case models.RoleReviewer:
		return doc.ReviewerID.Valid && doc.ReviewerID.Int64 == userID
	case models.RoleDeveloper:
		return doc.Status == models.StatusPublished
	default:
		return false
	}
}

func canWrite(doc *models.Document, userID int64, role models.UserRole) bool {
	switch role {
	case models.RoleAdmin:
		return true
	case models.RoleWriter:
		return doc.CreatedBy == userID
	default:
		return false
	}
}

// canEditMeta — кому разрешено менять метаданные документа (title,
// description, category, reviewer). Шире, чем canWrite: ревьюер тоже имеет
// право, потому что часто именно он формулирует точный заголовок при приёмке.
// Только developer не может редактировать.
func canEditMeta(doc *models.Document, userID int64, role models.UserRole) bool {
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
