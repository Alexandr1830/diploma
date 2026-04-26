package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sergi/go-diff/diffmatchpatch"

	"diploma/internal/models"
	"diploma/internal/parser"
	"diploma/internal/preview"
	"diploma/internal/repository"
)

var (
	ErrVersionNotFound = errors.New("version not found")
	ErrInvalidFileType = errors.New("invalid file type; allowed: docx, txt, md, yaml")
)

// filesDir — где на диске лежат загруженные файлы версий. В docker-compose
// сюда пробрасывается ./files с хоста.
const filesDir = "/files"

type versionService struct {
	docs   repository.DocumentRepository
	vers   repository.DocumentVersionRepository
	parser parser.Parser
}

func NewVersionService(
	docs repository.DocumentRepository,
	vers repository.DocumentVersionRepository,
	p parser.Parser,
) VersionService {
	return &versionService{docs: docs, vers: vers, parser: p}
}

// Upload загружает новую версию. Старая current сбрасывается, новая ставится
// текущей. Номер версии — порядковый, считается от количества уже существующих.
//
// Парсинг: если файл с именем req.FileName уже лежит в /files, парсер
// извлекает текст и проверяет наличие картинок. parsed_text и has_images
// заполняются автоматически. Если фронт явно прислал ParsedText — он в
// приоритете. Падение парсера не блокирует загрузку — версия просто сохранится
// без распарсенного текста.
func (s *versionService) Upload(ctx context.Context, docID int64, req models.UploadVersionRequest, userID int64, role string) (*models.DocumentVersion, error) {
	if _, ok := models.AllowedFileTypes[req.FileType]; !ok {
		return nil, ErrInvalidFileType
	}

	// file_path всегда строим сами как /files/<basename(file_name)>. Клиенту
	// не доверяем — иначе через ../../ можно было бы вылезти из /files.
	safeName := filepath.Base(req.FileName)
	safePath := filepath.Join(filesDir, safeName)

	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("versionService.Upload: get doc: %w", err)
	}
	if !canWrite(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	existing, err := s.vers.ListByDocument(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("versionService.Upload: list versions: %w", err)
	}
	versionNumber := strconv.Itoa(len(existing) + 1)

	if err := s.vers.UnsetCurrentForDocument(ctx, docID); err != nil {
		return nil, fmt.Errorf("versionService.Upload: unset current: %w", err)
	}

	v := &models.DocumentVersion{
		DocumentID:    docID,
		VersionNumber: versionNumber,
		FileName:      safeName,
		FilePath:      safePath,
		FileType:      req.FileType,
		UploadedBy:    userID,
		IsCurrent:     true,
	}

	parsedText := req.ParsedText
	hasImages := false
	if parsedText == "" && s.parser != nil {
		if pt, hi, ok := s.tryParse(safePath, req.FileType); ok {
			parsedText = pt
			hasImages = hi
		}
	}
	if parsedText != "" {
		v.ParsedText = sql.NullString{String: parsedText, Valid: true}
	}
	v.HasImages = hasImages
	if req.ChangeSummary != "" {
		v.ChangeSummary = sql.NullString{String: req.ChangeSummary, Valid: true}
	}

	if err := s.vers.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("versionService.Upload: create: %w", err)
	}

	if err := s.docs.SetCurrentVersion(ctx, docID, v.ID); err != nil {
		return nil, fmt.Errorf("versionService.Upload: set current on doc: %w", err)
	}

	// Заранее генерируем PDF-превью для docx, чтобы фронт мог сразу показать
	// файл в iframe без принудительного скачивания. Если soffice упал —
	// ничего страшного, исходный .docx по-прежнему доступен по ссылке.
	preview.Generate(safePath, req.FileType)

	return v, nil
}

// tryParse пытается распарсить файл по пути path. Возвращает ok=false на любой
// сбой (нет файла, парсер упал и т.д.) — чтобы загрузка не валилась 500-кой,
// а просто сохранила версию без parsed_text.
func (s *versionService) tryParse(path string, ft models.FileType) (string, bool, bool) {
	if _, err := os.Stat(path); err != nil {
		return "", false, false
	}
	res, err := s.parser.Parse(path, ft)
	if err != nil || res == nil {
		return "", false, false
	}
	return res.ParsedText, res.HasImages, true
}

// List возвращает все версии документа, доступные для чтения текущему пользователю.
func (s *versionService) List(ctx context.Context, docID int64, userID int64, role string) ([]models.DocumentVersion, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("versionService.List: get doc: %w", err)
	}
	if !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	versions, err := s.vers.ListByDocument(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("versionService.List: %w", err)
	}
	return versions, nil
}

// GetByID возвращает одну версию и заодно проверяет, что она реально относится
// к указанному документу (защита от подмены docID в URL).
func (s *versionService) GetByID(ctx context.Context, docID, versionID int64, userID int64, role string) (*models.DocumentVersion, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("versionService.GetByID: get doc: %w", err)
	}
	if !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("versionService.GetByID: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	return v, nil
}

// Restore делает старую версию текущей. Статус документа не трогает.
func (s *versionService) Restore(ctx context.Context, docID, versionID int64, userID int64, role string) (*models.DocumentVersion, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("versionService.Restore: get doc: %w", err)
	}
	// Право то же, что на загрузку: только writer-владелец или admin.
	if !canWrite(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	v, err := s.vers.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("versionService.Restore: get version: %w", err)
	}
	if v.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	if err := s.vers.UnsetCurrentForDocument(ctx, docID); err != nil {
		return nil, fmt.Errorf("versionService.Restore: unset current: %w", err)
	}
	if err := s.vers.SetCurrent(ctx, versionID); err != nil {
		return nil, fmt.Errorf("versionService.Restore: set current: %w", err)
	}
	if err := s.docs.SetCurrentVersion(ctx, docID, versionID); err != nil {
		return nil, fmt.Errorf("versionService.Restore: update doc: %w", err)
	}

	v.IsCurrent = true
	return v, nil
}

// Diff сравнивает parsed_text двух версий и заодно сообщает, поменялся ли
// флаг наличия картинок (бинарный признак — нет ли у одной версии картинок,
// а у другой нет).
func (s *versionService) Diff(ctx context.Context, docID, v1ID, v2ID, userID int64, role string) (*models.DiffResponse, error) {
	doc, err := s.docs.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("versionService.Diff: get doc: %w", err)
	}
	if !canRead(doc, userID, models.UserRole(role)) {
		return nil, ErrForbidden
	}

	ver1, err := s.vers.GetByID(ctx, v1ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("versionService.Diff: get v1: %w", err)
	}
	if ver1.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	ver2, err := s.vers.GetByID(ctx, v2ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("versionService.Diff: get v2: %w", err)
	}
	if ver2.DocumentID != docID {
		return nil, ErrVersionNotFound
	}

	text1 := ""
	if ver1.ParsedText.Valid {
		text1 = ver1.ParsedText.String
	}
	text2 := ""
	if ver2.ParsedText.Valid {
		text2 = ver2.ParsedText.String
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(text1, text2, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	chunks := make([]models.DiffChunk, 0, len(diffs))
	for _, d := range diffs {
		var typ string
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			typ = "added"
		case diffmatchpatch.DiffDelete:
			typ = "removed"
		case diffmatchpatch.DiffEqual:
			typ = "equal"
		}
		chunks = append(chunks, models.DiffChunk{Type: typ, Text: d.Text})
	}

	return &models.DiffResponse{
		TextDiff:      chunks,
		ImagesChanged: ver1.HasImages != ver2.HasImages,
	}, nil
}
