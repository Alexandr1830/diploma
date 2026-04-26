// Package parser достаёт текст и проверяет наличие изображений в загруженных
// файлах поддерживаемых форматов: DOCX, TXT, MD, YAML.
//
// Снаружи виден только интерфейс Parser. Внутри по FileType выбирается нужная
// реализация: DOCX парсится через распаковку zip и обход XML, текстовые форматы
// читаются как UTF-8.
package parser

import (
	"errors"
	"fmt"

	"diploma/internal/models"
)

// ParseResult — результат парсинга. HasImages имеет смысл только для DOCX,
// для текстовых форматов всегда false.
type ParseResult struct {
	ParsedText string
	HasImages  bool
}

type Parser interface {
	Parse(path string, ft models.FileType) (*ParseResult, error)
}

var ErrUnsupportedFileType = errors.New("parser: unsupported file type")

type defaultParser struct{}

func New() Parser {
	return &defaultParser{}
}

func (p *defaultParser) Parse(path string, ft models.FileType) (*ParseResult, error) {
	switch ft {
	case models.FileTypeDocx:
		return parseDOCX(path)
	case models.FileTypeTXT, models.FileTypeMD, models.FileTypeYAML:
		return parseText(path)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFileType, ft)
	}
}
