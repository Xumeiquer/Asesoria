// Package library manages the user's document library: an organised folder
// tree on the host, incremental scanning, text extraction and classification.
package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/store"
)

const (
	SettingRoot     = "library_root"
	SettingOCRLang  = "library_ocr_lang"
	SettingLastScan = "library_last_scan"
)

// Service reads its configuration from the settings table on every call so the
// root can be changed without restarting the server.
type Service struct {
	Store *store.Store
}

func New(st *store.Store) *Service {
	return &Service{Store: st}
}

type Config struct {
	Root       string      `json:"root"`
	Configured bool        `json:"configured"`
	Writable   bool        `json:"writable"`
	OCRLang    string      `json:"ocr_lang"`
	Years      []int       `json:"years"`
	Tesseract  bool        `json:"tesseract"`
	PDFText    bool        `json:"pdf_text"`
	PDFImages  bool        `json:"pdf_images"`
	LastScan   *ScanResult `json:"last_scan,omitempty"`
}

func (s *Service) Config(ctx context.Context) (Config, error) {
	cfg := Config{OCRLang: "spa"}
	cfg.Tesseract = hasCommand("tesseract")
	cfg.PDFText = hasCommand("pdftotext")
	cfg.PDFImages = cfg.Tesseract && hasCommand("pdftoppm")

	root, err := s.Root(ctx)
	if err != nil {
		return cfg, err
	}
	cfg.Root = root
	cfg.Configured = root != ""
	if root != "" {
		cfg.Writable = writable(root)
		cfg.Years = Years()
	}
	if lang, err := s.Store.GetSetting(ctx, SettingOCRLang); err == nil && lang != "" {
		cfg.OCRLang = lang
	}
	if raw, err := s.Store.GetSetting(ctx, SettingLastScan); err == nil && raw != "" {
		var last ScanResult
		if err := json.Unmarshal([]byte(raw), &last); err == nil {
			cfg.LastScan = &last
		}
	}
	return cfg, nil
}

// UpdateConfig validates and persists the library root. An empty root disables
// the managed library without deleting any files.
func (s *Service) UpdateConfig(ctx context.Context, rawRoot, ocrLang string) (Config, error) {
	root, err := NormalizeRoot(rawRoot)
	if err != nil {
		return Config{}, err
	}
	if root != "" {
		if err := os.MkdirAll(root, 0o750); err != nil {
			return Config{}, fmt.Errorf("no se pudo crear la carpeta: %w", err)
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return Config{}, errors.New("la ruta debe ser una carpeta")
		}
		if !writable(root) {
			return Config{}, errors.New("la carpeta no tiene permisos de escritura")
		}
	}
	if ocrLang = strings.TrimSpace(ocrLang); ocrLang == "" {
		ocrLang = "spa"
	}
	if err := s.Store.SetSetting(ctx, SettingRoot, root); err != nil {
		return Config{}, err
	}
	if err := s.Store.SetSetting(ctx, SettingOCRLang, ocrLang); err != nil {
		return Config{}, err
	}
	if root != "" {
		if _, err := s.EnsureTree(ctx); err != nil {
			return Config{}, err
		}
	}
	return s.Config(ctx)
}

func (s *Service) Root(ctx context.Context) (string, error) {
	root, err := s.Store.GetSetting(ctx, SettingRoot)
	if errors.Is(err, store.ErrNotFound) {
		return "", nil
	}
	return root, err
}

func (s *Service) OCRLang(ctx context.Context) string {
	lang, err := s.Store.GetSetting(ctx, SettingOCRLang)
	if err != nil || strings.TrimSpace(lang) == "" {
		return "spa"
	}
	return lang
}

// NormalizeRoot expands ~ and makes the path absolute.
func NormalizeRoot(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw == "~" || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("no se pudo resolver el directorio personal: %w", err)
		}
		if raw == "~" {
			raw = home
		} else {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "~/"))
		}
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if abs == string(filepath.Separator) {
		return "", errors.New("no se puede usar la raíz del sistema como biblioteca")
	}
	return abs, nil
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".asesoria-write-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
