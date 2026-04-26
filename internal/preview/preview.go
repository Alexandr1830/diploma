// Package preview генерирует PDF-копию исходного документа рядом с самим
// файлом. Нужно для того, чтобы фронт мог показать содержимое в <iframe> без
// принудительного скачивания: браузер качает .docx, но рендерит .pdf
// нативно.
//
// PDF лежит по пути <originalPath>.preview.pdf. Суффикс добавляется поверх
// исходного имени, чтобы избежать коллизий, если у двух файлов совпадает
// базовое имя но разное расширение.
package preview

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"diploma/internal/models"
)

const (
	// previewSuffix приклеивается к исходному пути:
	//   /files/spec_v1.docx → /files/spec_v1.docx.preview.pdf
	previewSuffix = ".preview.pdf"

	// soffice может зависнуть на кривом docx — ставим жёсткий таймаут.
	conversionTimeout = 60 * time.Second
)

// Path возвращает путь, где должен лежать PDF-превью для данного исходника.
// Чистая функция, файловую систему не трогает — для проверки наличия используй Exists.
func Path(sourcePath string) string {
	return sourcePath + previewSuffix
}

// Exists проверяет, есть ли уже сгенерированный PDF для исходного файла.
func Exists(sourcePath string) bool {
	_, err := os.Stat(Path(sourcePath))
	return err == nil
}

// Generate вызывает LibreOffice headless и кладёт результат в Path(sourcePath).
// Конвертируется только docx — остальные форматы (txt/md/yaml) фронт умеет
// показывать сам, для них функция возвращает false без ошибки.
//
// Любой сбой возвращает ok=false и не валит загрузку версии — для пользователя
// просто не будет inline-превью, файл всё равно скачается по обычной ссылке.
func Generate(sourcePath string, ft models.FileType) bool {
	if ft != models.FileTypeDocx {
		return false
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), conversionTimeout)
	defer cancel()

	dir := filepath.Dir(sourcePath)
	// soffice кладёт <basename без расширения>.pdf в --outdir. После этого
	// переименовываем результат в <fullbasename>.preview.pdf, чтобы он лежал
	// рядом с исходником и его можно было найти по convention.
	cmd := exec.CommandContext(ctx, "soffice",
		"--headless",
		"--convert-to", "pdf",
		"--outdir", dir,
		sourcePath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "preview.Generate: soffice failed for %s: %v (stderr=%q)\n",
			sourcePath, err, stderr.String())
		return false
	}

	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	stripped := strings.TrimSuffix(base, ext)
	defaultOutput := filepath.Join(dir, stripped+".pdf")

	target := Path(sourcePath)
	if defaultOutput == target {
		// Крайний случай: исходник уже называется *.preview.pdf — soffice
		// перезапишет тот же путь. Просто выходим.
		return true
	}
	if err := os.Rename(defaultOutput, target); err != nil {
		fmt.Fprintf(os.Stderr, "preview.Generate: rename %s → %s failed: %v\n",
			defaultOutput, target, err)
		return false
	}
	return true
}
