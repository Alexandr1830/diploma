package parser

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// maxTextSize — потолок на размер читаемого файла. Защита от того, что кто-то
// загрузит 200-мегабайтный txt и положит память сервиса.
const maxTextSize = 5 * 1024 * 1024 // 5 MB

// parseText читает файл как UTF-8. Подходит для txt, md, yaml. Изображений
// в этих форматах быть не может, поэтому HasImages=false.
func parseText(path string) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parser.text: open: %w", err)
	}
	defer f.Close()

	buf, err := io.ReadAll(io.LimitReader(f, maxTextSize))
	if err != nil {
		return nil, fmt.Errorf("parser.text: read: %w", err)
	}
	return &ParseResult{
		ParsedText: strings.TrimSpace(string(buf)),
		HasImages:  false,
	}, nil
}
