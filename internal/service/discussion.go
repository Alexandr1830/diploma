package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"diploma/internal/models"
	"diploma/internal/repository"
)

var (
	ErrThreadNotFound       = errors.New("thread not found")
	ErrThreadResolved       = errors.New("thread is resolved")
	ErrInvalidThreadType    = errors.New("invalid thread type; allowed: general, anchored")
	ErrEmptyMessage         = errors.New("message must not be empty")
	ErrDocumentNotPublished = errors.New("document is not published")
)

type discussionService struct {
	docs    repository.DocumentRepository
	vers    repository.DocumentVersionRepository
	threads repository.DiscussionThreadRepository
	msgs    repository.DiscussionMessageRepository
}

func NewDiscussionService(
	docs repository.DocumentRepository,
	vers repository.DocumentVersionRepository,
	threads repository.DiscussionThreadRepository,
	msgs repository.DiscussionMessageRepository,
) DiscussionService {
	return &discussionService{docs: docs, vers: vers, threads: threads, msgs: msgs}
}

// CreateThread создаёт тред на конкретной версии документа. Право — у того,
// кто вообще видит документ (writer-владелец, назначенный ревьюер, admin).
func (s *discussionService) CreateThread(ctx context.Context, docID, versionID int64, req models.CreateThreadRequest, userID int64, role string) (*models.DiscussionThread, error) {
	if req.Type != models.ThreadTypeGeneral && req.Type != models.ThreadTypeAnchored {
		return nil, ErrInvalidThreadType
	}

	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.CreateThread: get doc: %w", err)
	}

	if !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("discussionService.CreateThread: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	t := &models.DiscussionThread{
		DocumentID: docID,
		VersionID:  versionID,
		CreatedBy:  userID,
		ThreadType: req.Type,
		Status:     models.ThreadStatusOpen,
	}
	if req.Anchor != nil {
		t.AnchorText = sql.NullString{String: *req.Anchor, Valid: true}
	}

	if err := s.threads.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("discussionService.CreateThread: create: %w", err)
	}
	return t, nil
}

// ListThreads возвращает треды одной конкретной версии.
func (s *discussionService) ListThreads(ctx context.Context, docID, versionID int64, userID int64, role string) ([]models.DiscussionThread, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.ListThreads: get doc: %w", err)
	}

	if !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("discussionService.ListThreads: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	threads, err := s.threads.ListByVersion(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("discussionService.ListThreads: %w", err)
	}
	return threads, nil
}

// CreateMessage добавляет сообщение в открытый тред.
//   - внутренний тред (is_public=false): writer-владелец, ревьюер или admin
//   - публичный тред библиотеки: любой залогиненный пользователь
//
// В закрытый (resolved) тред писать нельзя — вернётся ErrThreadResolved.
func (s *discussionService) CreateMessage(ctx context.Context, threadID int64, text string, userID int64, role string) (*models.DiscussionMessage, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyMessage
	}

	t, err := s.threads.GetByID(ctx, threadID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrThreadNotFound
		}
		return nil, fmt.Errorf("discussionService.CreateMessage: get thread: %w", err)
	}

	if t.Status == models.ThreadStatusResolved {
		return nil, ErrThreadResolved
	}

	doc, err := s.docs.GetByID(ctx, t.DocumentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.CreateMessage: get doc: %w", err)
	}

	if !t.IsPublic && !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	m := &models.DiscussionMessage{
		ThreadID:    threadID,
		AuthorID:    userID,
		MessageText: text,
	}
	if err := s.msgs.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("discussionService.CreateMessage: create: %w", err)
	}
	return m, nil
}

// ListMessages — все сообщения треда. Публичные треды читает любой
// залогиненный, внутренние — только участники (см. canRead).
func (s *discussionService) ListMessages(ctx context.Context, threadID int64, userID int64, role string) ([]models.DiscussionMessage, error) {
	t, err := s.threads.GetByID(ctx, threadID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrThreadNotFound
		}
		return nil, fmt.Errorf("discussionService.ListMessages: get thread: %w", err)
	}

	doc, err := s.docs.GetByID(ctx, t.DocumentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.ListMessages: get doc: %w", err)
	}

	if !t.IsPublic && !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	msgs, err := s.msgs.ListByThread(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("discussionService.ListMessages: %w", err)
	}
	return msgs, nil
}

// ResolveThread закрывает тред. Внутренние закрывает ревьюер или admin,
// публичные — только admin (writer не должен по своему усмотрению затыкать
// обсуждение в библиотеке).
func (s *discussionService) ResolveThread(ctx context.Context, threadID int64, userID int64, role string) (*models.DiscussionThread, error) {
	t, err := s.threads.GetByID(ctx, threadID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrThreadNotFound
		}
		return nil, fmt.Errorf("discussionService.ResolveThread: get thread: %w", err)
	}

	if t.Status == models.ThreadStatusResolved {
		return nil, ErrThreadResolved
	}

	doc, err := s.docs.GetByID(ctx, t.DocumentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.ResolveThread: get doc: %w", err)
	}

	if t.IsPublic {
		if models.UserRole(role) != models.RoleAdmin {
			return nil, ErrForbidden
		}
	} else if !canReview(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	if err := s.threads.UpdateStatus(ctx, threadID, models.ThreadStatusResolved); err != nil {
		return nil, fmt.Errorf("discussionService.ResolveThread: update status: %w", err)
	}

	// Перечитываем тред, чтобы вернуть resolved_at, который проставила БД.
	t, err = s.threads.GetByID(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("discussionService.ResolveThread: re-fetch: %w", err)
	}
	return t, nil
}

// GetDiscussionView — агрегирующий эндпоинт для страницы обсуждения. Один
// запрос вместо трёх (документ + версия + треды + сообщения). Включает треды
// со ВСЕХ версий документа: загрузка новой версии не должна прятать ранее
// открытые треды. Публичные треды библиотеки сюда не попадают — они живут на
// /library/:id.
func (s *discussionService) GetDiscussionView(ctx context.Context, docID, userID int64, role string) (*models.DiscussionViewResponse, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.GetDiscussionView: get doc: %w", err)
	}

	if !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	resp := &models.DiscussionViewResponse{
		Document: doc,
		Threads:  []models.ThreadWithMessages{},
	}

	if doc.CurrentVersionID.Valid {
		ver, err := s.vers.GetByID(ctx, doc.CurrentVersionID.Int64)
		if err == nil {
			resp.CurrentVersion = ver
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("discussionService.GetDiscussionView: get version: %w", err)
		}
	}

	threads, err := s.threads.ListByDocument(ctx, docID, false)
	if err != nil {
		return nil, fmt.Errorf("discussionService.GetDiscussionView: list threads: %w", err)
	}

	for _, t := range threads {
		msgs, err := s.msgs.ListByThread(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("discussionService.GetDiscussionView: list messages for thread %d: %w", t.ID, err)
		}

		tw := models.ThreadWithMessages{
			DiscussionThread: t,
			MessagesCount:    len(msgs),
			Messages:         msgs,
		}

		if len(msgs) > 0 {
			last := msgs[len(msgs)-1]
			tw.LastMessageAt = &last.CreatedAt
			preview := last.MessageText
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			tw.LastMessagePreview = preview
		}

		resp.Threads = append(resp.Threads, tw)
	}

	sortByRecent(resp.Threads)
	return resp, nil
}

// CreateLibraryThread открывает публичный тред на опубликованном документе.
// Писать может любой залогиненный пользователь. Тред закреплён за текущей
// published_version_id — даже если позже выйдет новая публикация, тред
// продолжит ссылаться на ту версию, по которой шло обсуждение.
func (s *discussionService) CreateLibraryThread(ctx context.Context, docID int64, req models.CreateThreadRequest, userID int64) (*models.DiscussionThread, error) {
	if req.Type != models.ThreadTypeGeneral && req.Type != models.ThreadTypeAnchored {
		return nil, ErrInvalidThreadType
	}
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.CreateLibraryThread: get doc: %w", err)
	}
	if doc.Status != models.StatusPublished {
		return nil, ErrDocumentNotPublished
	}
	if !doc.PublishedVersionID.Valid {
		return nil, ErrDocumentNotPublished
	}

	t := &models.DiscussionThread{
		DocumentID: docID,
		VersionID:  doc.PublishedVersionID.Int64,
		CreatedBy:  userID,
		ThreadType: req.Type,
		Status:     models.ThreadStatusOpen,
		IsPublic:   true,
	}
	if req.Anchor != nil {
		t.AnchorText = sql.NullString{String: *req.Anchor, Valid: true}
	}
	if err := s.threads.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("discussionService.CreateLibraryThread: create: %w", err)
	}
	return t, nil
}

// ListLibraryThreads — публичные треды опубликованного документа вместе с
// сообщениями. Видно любому залогиненному.
func (s *discussionService) ListLibraryThreads(ctx context.Context, docID int64) ([]models.ThreadWithMessages, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("discussionService.ListLibraryThreads: get doc: %w", err)
	}
	if doc.Status != models.StatusPublished {
		return nil, ErrDocumentNotPublished
	}

	threads, err := s.threads.ListPublicByDocument(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("discussionService.ListLibraryThreads: list: %w", err)
	}

	out := []models.ThreadWithMessages{}
	for _, t := range threads {
		msgs, err := s.msgs.ListByThread(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("discussionService.ListLibraryThreads: messages for %d: %w", t.ID, err)
		}
		tw := models.ThreadWithMessages{
			DiscussionThread: t,
			MessagesCount:    len(msgs),
			Messages:         msgs,
		}
		if len(msgs) > 0 {
			last := msgs[len(msgs)-1]
			tw.LastMessageAt = &last.CreatedAt
			preview := last.MessageText
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			tw.LastMessagePreview = preview
		}
		out = append(out, tw)
	}
	sortByRecent(out)
	return out, nil
}

// sortByRecent — треды с недавними сообщениями вверх, треды без сообщений
// сортируются по created_at от новых к старым.
func sortByRecent(threads []models.ThreadWithMessages) {
	sort.Slice(threads, func(i, j int) bool {
		ti := threads[i].LastMessageAt
		tj := threads[j].LastMessageAt
		if ti == nil && tj == nil {
			return threads[i].CreatedAt.After(threads[j].CreatedAt)
		}
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.After(*tj)
	})
}
