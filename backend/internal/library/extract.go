package library

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	OCRText        = "text"
	OCRDone        = "ocr"
	OCRNone        = "none"
	OCRUnavailable = "unavailable"
	OCRUnsupported = "unsupported"
	OCRError       = "error"

	maxTextBytes = 16 << 20
	maxTextRunes = 200_000
)

var supportedExt = map[string]bool{
	".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".tif": true,
	".tiff": true, ".webp": true, ".bmp": true, ".txt": true, ".md": true,
	".markdown": true, ".csv": true, ".docx": true, ".xlsx": true,
	".html": true, ".htm": true, ".xml": true, ".json": true,
}

var imageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".tif": true,
	".tiff": true, ".webp": true, ".bmp": true,
}

func Supported(path string) bool {
	return supportedExt[strings.ToLower(filepath.Ext(path))]
}

// ExtractFile returns plain text plus an OCR status. It uses local command
// line tools when available and never sends content to an external service.
func ExtractFile(ctx context.Context, path, lang string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if !supportedExt[ext] {
		return "", OCRUnsupported, nil
	}
	switch {
	case ext == ".pdf":
		return extractPDF(ctx, path, lang)
	case imageExt[ext]:
		if !hasCommand("tesseract") {
			return "", OCRUnavailable, nil
		}
		text, err := ocrImage(ctx, path, lang)
		if err != nil {
			return "", OCRError, err
		}
		return cleanText(text), OCRDone, nil
	case ext == ".docx":
		text, err := extractZipXML(path, "word/document.xml")
		return cleanText(text), OCRText, err
	case ext == ".xlsx":
		text, err := extractXLSX(path)
		return cleanText(text), OCRText, err
	default:
		text, err := readPlain(path)
		if ext == ".html" || ext == ".htm" || ext == ".xml" {
			text = stripTags(text)
		}
		return cleanText(text), OCRText, err
	}
}

func extractPDF(ctx context.Context, path, lang string) (string, string, error) {
	var embedded string
	if hasCommand("pdftotext") {
		out, err := run(ctx, "pdftotext", "-layout", "-enc", "UTF-8", path, "-")
		if err == nil {
			embedded = cleanText(out)
			if meaningful(embedded) {
				return embedded, OCRText, nil
			}
		}
	}
	if hasCommand("pdftoppm") && hasCommand("tesseract") {
		text, err := ocrPDF(ctx, path, lang)
		if err == nil && meaningful(text) {
			return cleanText(text), OCRDone, nil
		}
		if err != nil && embedded == "" {
			return "", OCRError, err
		}
	}
	if embedded != "" {
		return embedded, OCRText, nil
	}
	if !hasCommand("pdftotext") && !(hasCommand("pdftoppm") && hasCommand("tesseract")) {
		return "", OCRUnavailable, nil
	}
	return "", OCRUnavailable, nil
}

func ocrPDF(ctx context.Context, path, lang string) (string, error) {
	dir, err := os.MkdirTemp("", "asesoria-ocr-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	prefix := filepath.Join(dir, "page")
	if _, err := run(ctx, "pdftoppm", "-r", "200", "-png", "-f", "1", "-l", "10", path, prefix); err != nil {
		return "", err
	}
	files, _ := filepath.Glob(prefix + "*.png")
	sort.Strings(files)
	if len(files) == 0 {
		return "", fmt.Errorf("no se pudieron rasterizar las páginas del PDF")
	}
	var parts []string
	for _, image := range files {
		text, err := ocrImage(ctx, image, lang)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n"), nil
}

func ocrImage(ctx context.Context, path, lang string) (string, error) {
	if lang == "" {
		lang = "spa"
	}
	out, err := run(ctx, "tesseract", path, "stdout", "-l", lang)
	if err == nil {
		return out, nil
	}
	if lang != "eng" {
		if out, retry := run(ctx, "tesseract", path, "stdout", "-l", "eng"); retry == nil {
			return out, nil
		}
	}
	return run(ctx, "tesseract", path, "stdout")
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s: %s", name, msg)
	}
	return string(out), nil
}

func extractZipXML(path, entry string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != entry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		body, err := io.ReadAll(io.LimitReader(rc, maxTextBytes))
		if err != nil {
			return "", err
		}
		text := string(body)
		text = strings.NewReplacer("</w:p>", "\n", "</a:p>", "\n", "<w:tab/>", "\t").Replace(text)
		return stripTags(text), nil
	}
	return "", fmt.Errorf("no se encontró %s", entry)
}

func extractXLSX(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	var parts []string
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if !strings.HasPrefix(name, "xl/sharedstrings.xml") && !strings.HasPrefix(name, "xl/worksheets/sheet") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxTextBytes))
		rc.Close()
		if err != nil {
			return "", err
		}
		for _, match := range regexp.MustCompile(`(?s)<t[^>]*>(.*?)</t>`).FindAllStringSubmatch(string(body), -1) {
			if text := strings.TrimSpace(html.UnescapeString(stripTags(match[1]))); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n"), nil
}

func readPlain(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, maxTextBytes))
	if err != nil {
		return "", err
	}
	if utf8.Valid(body) {
		return string(body), nil
	}
	// Spanish legacy files are often Windows-1252/ISO-8859-1. A direct
	// byte-to-rune conversion is enough for search and classification.
	runes := make([]rune, len(body))
	for i, b := range body {
		runes[i] = rune(b)
	}
	return string(runes), nil
}

var tagRegexp = regexp.MustCompile(`<[^>]+>`)

func stripTags(value string) string {
	return html.UnescapeString(tagRegexp.ReplaceAllString(value, " "))
}

func cleanText(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, line)
	}
	value = strings.TrimSpace(strings.Join(out, "\n"))
	runes := []rune(value)
	if len(runes) > maxTextRunes {
		value = string(runes[:maxTextRunes])
	}
	return value
}

func meaningful(text string) bool {
	return len([]rune(strings.TrimSpace(text))) >= 30
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
