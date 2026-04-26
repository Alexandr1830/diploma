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
	ErrWrongStatus      = errors.New("document status does not allow this operation")
	ErrNoReviewer       = errors.New("document has no reviewer assigned")
	ErrNoCurrentVersion = errors.New("document has no current version")
	ErrNoteRequired     = errors.New("note is required for request_revision")
)

type reviewService struct {
	docs     repository.DocumentRepository
	vers     repository.DocumentVersionRepository
	comments repository.ReviewCommentRepository
	actions  repository.ReviewActionRepository
	threads  repository.DiscussionThreadRepository
	msgs     repository.DiscussionMessageRepository
}

// NewReviewService принимает ещё и репо обсуждений, потому что RequestRevision
// автоматически постит замечание ревьюера в тред — так автор видит замечание
// прямо в чате документа, а не должен лезть в историю review_actions.
func NewReviewService(
	docs repository.DocumentRepository,
	vers repository.DocumentVersionRepository,
	comments repository.ReviewCommentRepository,
	actions repository.ReviewActionRepository,
	threads repository.DiscussionThreadRepository,
	msgs repository.DiscussionMessageRepository,
) ReviewService {
	return &reviewService{docs: docs, vers: vers, comments: comments, actions: actions, threads: threads, msgs: msgs}
}

// canReview — назначенный ревьюер или admin.
func canReview(doc *models.Document, userID int64, role models.UserRole) bool {
	switch role {
	case models.RoleAdmin:
		return true
	case models.RoleReviewer:
		return doc.ReviewerID.Valid && doc.ReviewerID.Int64 == userID
	default:
		return false
	}
}

// canReadReview — writer-владелец, назначенный ревьюер или admin. Шире, чем
// canReview, потому что комментарии и историю действий писатель тоже должен
// видеть, не только править их.
func canReadReview(doc *models.Document, userID int64, role models.UserRole) bool {
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

// Submit переводит документ из draft/needs_revision → in_review. Делать может
// только writer-владелец или admin. Перед отправкой обязан быть назначен
// ревьюер.
func (s *reviewService) Submit(ctx context.Context, docID, userID int64, role string) (*models.Document, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("reviewService.Submit: get doc: %w", err)
	}

	r := models.UserRole(role)
	if r != models.RoleAdmin && !(r == models.RoleWriter && doc.CreatedBy == userID) {
		return nil, ErrForbidden
	}

	if !doc.ReviewerID.Valid {
		return nil, ErrNoReviewer
	}

	if doc.Status != models.StatusDraft && doc.Status != models.StatusNeedsRevision {
		return nil, ErrWrongStatus
	}

	if err := s.docs.UpdateStatus(ctx, docID, models.StatusInReview); err != nil {
		return nil, fmt.Errorf("reviewService.Submit: update status: %w", err)
	}
	doc.Status = models.StatusInReview
	return doc, nil
}

// Approve переводит документ из in_review → approved и пишет запись в
// review_actions. Делать может только назначенный ревьюер или admin.
func (s *reviewService) Approve(ctx context.Context, docID, userID int64, role string) (*models.ReviewActionRecord, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("reviewService.Approve: get doc: %w", err)
	}

	if !canReview(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	if doc.Status != models.StatusInReview {
		return nil, ErrWrongStatus
	}

	if !doc.CurrentVersionID.Valid {
		return nil, ErrNoCurrentVersion
	}

	if err := s.docs.UpdateStatus(ctx, docID, models.StatusApproved); err != nil {
		return nil, fmt.Errorf("reviewService.Approve: update status: %w", err)
	}

	a := &models.ReviewActionRecord{
		DocumentID: docID,
		VersionID:  doc.CurrentVersionID.Int64,
		ReviewerID: userID,
		Action:     models.ReviewActionApprove,
		Note:       "",
	}
	if err := s.actions.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("reviewService.Approve: create action: %w", err)
	}
	return a, nil
}

// RequestRevision переводит документ из in_review → needs_revision и пишет
// действие в review_actions. note должен быть непустой — иначе непонятно, что
// именно писатель должен переделать.
func (s *reviewService) RequestRevision(ctx context.Context, docID int64, note string, userID int64, role string) (*models.ReviewActionRecord, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("reviewService.RequestRevision: get doc: %w", err)
	}

	if !canReview(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	if doc.Status != models.StatusInReview {
		return nil, ErrWrongStatus
	}

	if note == "" {
		return nil, ErrNoteRequired
	}

	if !doc.CurrentVersionID.Valid {
		return nil, ErrNoCurrentVersion
	}

	if err := s.docs.UpdateStatus(ctx, docID, models.StatusNeedsRevision); err != nil {
		return nil, fmt.Errorf("reviewService.RequestRevision: update status: %w", err)
	}

	a := &models.ReviewActionRecord{
		DocumentID: docID,
		VersionID:  doc.CurrentVersionID.Int64,
		ReviewerID: userID,
		Action:     models.ReviewActionRequestRevision,
		Note:       note,
	}
	if err := s.actions.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("reviewService.RequestRevision: create action: %w", err)
	}

	// Дублируем замечание в обсуждение: создаём общий тред на текущей версии и
	// первым сообщением кладём note. Если запись треда упадёт — статус и
	// review_action уже сохранены, так что не страшно. Просто писатель не
	// увидит уведомление в чате и узнает о замечании по статусу документа.
	if s.threads != nil && s.msgs != nil {
		t := &models.DiscussionThread{
			DocumentID: docID,
			VersionID:  doc.CurrentVersionID.Int64,
			CreatedBy:  userID,
			ThreadType: models.ThreadTypeGeneral,
			Status:     models.ThreadStatusOpen,
			IsPublic:   false,
		}
		if err := s.threads.Create(ctx, t); err == nil {
			m := &models.DiscussionMessage{
				ThreadID:    t.ID,
				AuthorID:    userID,
				MessageText: "Запрос на доработку: " + note,
			}
			_ = s.msgs.Create(ctx, m)
		}
	}

	return a, nil
}

// AddComment добавляет комментарий ревьюера к конкретной версии. Доступно
// только назначенному ревьюеру или admin.
func (s *reviewService) AddComment(ctx context.Context, docID, versionID int64, text string, userID int64, role string) (*models.ReviewComment, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("reviewService.AddComment: get doc: %w", err)
	}

	if !canReview(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("reviewService.AddComment: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	c := &models.ReviewComment{
		DocumentID:  docID,
		VersionID:   versionID,
		AuthorID:    userID,
		CommentText: text,
	}
	if err := s.comments.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("reviewService.AddComment: create: %w", err)
	}
	return c, nil
}

// ListComments — все комментарии к одной версии. Видны writer-владельцу,
// назначенному ревьюеру и admin'у.
func (s *reviewService) ListComments(ctx context.Context, docID, versionID int64, userID int64, role string) ([]models.ReviewComment, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("reviewService.ListComments: get doc: %w", err)
	}

	if !canReadReview(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("reviewService.ListComments: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	comments, err := s.comments.ListByVersion(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("reviewService.ListComments: %w", err)
	}
	return comments, nil
}

// ListActions — история approve/revision по документу. Те же права на чтение,
// что и у комментариев.
func (s *reviewService) ListActions(ctx context.Context, docID, userID int64, role string) ([]models.ReviewActionRecord, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("reviewService.ListActions: get doc: %w", err)
	}

	if !canReadReview(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	actions, err := s.actions.ListByDocument(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("reviewService.ListActions: %w", err)
	}
	return actions, nil
}
