package parser

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// parseDOCX вытаскивает текст из word/document.xml и параллельно проверяет,
// есть ли в архиве картинки (папка word/media/).
//
// DOCX — это zip-архив, внутри которого WordprocessingML. Текст лежит в
// <w:t>...</w:t>, абзацы — <w:p>, перенос строки — <w:br>, таб — <w:tab>.
// Парсер достаточно простой: на каждый <w:p> ставим \n, на <w:br> тоже \n,
// на <w:tab> — \t, остальное игнорируется.
func parseDOCX(path string) (*ParseResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("parser.docx: open zip: %w", err)
	}
	defer zr.Close()

	var documentFile *zip.File
	hasImages := false
	for _, f := range zr.File {
		switch {
		case f.Name == "word/document.xml":
			documentFile = f
		case strings.HasPrefix(f.Name, "word/media/"):
			hasImages = true
		}
	}
	if documentFile == nil {
		return nil, fmt.Errorf("parser.docx: word/document.xml not found")
	}

	rc, err := documentFile.Open()
	if err != nil {
		return nil, fmt.Errorf("parser.docx: open document.xml: %w", err)
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var sb strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parser.docx: xml decode: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "p":
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
			case "br":
				sb.WriteByte('\n')
			case "tab":
				sb.WriteByte('\t')
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}

	return &ParseResult{
		ParsedText: strings.TrimSpace(sb.String()),
		HasImages:  hasImages,
	}, nil
}
