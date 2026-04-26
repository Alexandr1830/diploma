package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"diploma/internal/models"
	"diploma/internal/repository"
)

type libraryService struct {
	docs repository.DocumentRepository
	vers repository.DocumentVersionRepository
}

func NewLibraryService(docs repository.DocumentRepository, vers repository.DocumentVersionRepository) LibraryService {
	return &libraryService{docs: docs, vers: vers}
}

// List — все опубликованные документы. Видны любому залогиненному пользователю.
func (s *libraryService) List(ctx context.Context) ([]models.Document, error) {
	docs, err := s.docs.ListPublished(ctx)
	if err != nil {
		return nil, fmt.Errorf("libraryService.List: %w", err)
	}
	return docs, nil
}

// GetByID — опубликованный документ вместе с published-версией. Один запрос
// вместо двух, чтобы фронт мог сразу отрисовать iframe с файлом.
func (s *libraryService) GetByID(ctx context.Context, id, userID int64, role string) (*models.LibraryDocumentResponse, error) {
	doc, err := s.docs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("libraryService.GetByID: %w", err)
	}
	if doc.Status != models.StatusPublished {
		return nil, ErrDocumentNotFound
	}

	resp := &models.LibraryDocumentResponse{Document: doc}
	if doc.PublishedVersionID.Valid {
		ver, err := s.vers.GetByID(ctx, doc.PublishedVersionID.Int64)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("libraryService.GetByID: get version: %w", err)
		}
		if err == nil {
			resp.PublishedVersion = ver
		}
	}
	return resp, nil
}
