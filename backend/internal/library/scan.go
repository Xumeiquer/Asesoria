package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
)

var (
	ErrDisabled  = errors.New("la biblioteca documental no está configurada")
	ErrDuplicate = errors.New("el documento ya existe en la biblioteca")
)

type ScanProblem struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type ScanResult struct {
	Root        string        `json:"root"`
	StartedAt   string        `json:"started_at"`
	FinishedAt  string        `json:"finished_at"`
	Scanned     int           `json:"scanned"`
	Imported    int           `json:"imported"`
	Duplicates  int           `json:"duplicates"`
	Skipped     int           `json:"skipped"`
	Errors      int           `json:"errors"`
	NeedsReview int           `json:"needs_review"`
	Truncated   bool          `json:"truncated"`
	Problems    []ScanProblem `json:"problems,omitempty"`
}

// Scan walks the configured library incrementally. It never deletes documents
// when a source file disappears.
func (s *Service) Scan(ctx context.Context, activeTaxpayerID string) (ScanResult, error) {
	root, err := s.Root(ctx)
	if err != nil {
		return ScanResult{}, err
	}
	if root == "" {
		return ScanResult{}, ErrDisabled
	}
	if _, err := s.EnsureTree(ctx); err != nil {
		return ScanResult{}, err
	}
	taxpayers, err := s.Store.ListTaxpayers(ctx)
	if err != nil {
		return ScanResult{}, err
	}
	var active *domain.Taxpayer
	for i := range taxpayers {
		if taxpayers[i].ID == activeTaxpayerID {
			active = &taxpayers[i]
			break
		}
	}
	knownPaths, err := s.Store.LibraryPaths(ctx)
	if err != nil {
		return ScanResult{}, err
	}
	states, err := s.Store.LibraryFiles(ctx)
	if err != nil {
		return ScanResult{}, err
	}
	lang := s.OCRLang(ctx)

	res := ScanResult{Root: root, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	absoluteKnown := make(map[string]bool, len(knownPaths))
	for p := range knownPaths {
		absoluteKnown[filepath.Clean(filepath.Join(root, filepath.FromSlash(p)))] = true
	}

	const maxFiles = 1000
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			res.Errors++
			res.Problems = append(res.Problems, ScanProblem{Path: path, Error: walkErr.Error()})
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			lower := strings.ToLower(name)
			if strings.HasPrefix(name, ".") || lower == "node_modules" || lower == "$recycle.bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, "~$") || strings.HasPrefix(name, ".asesoria-") {
			return nil
		}
		if res.Scanned >= maxFiles {
			res.Truncated = true
			return fs.SkipAll
		}
		res.Scanned++
		if !Supported(path) {
			res.Skipped++
			return nil
		}
		resolved := filepath.Clean(path)
		if absoluteKnown[resolved] {
			res.Skipped++
			return nil
		}
		info, err := d.Info()
		if err != nil {
			res.Errors++
			res.Problems = append(res.Problems, ScanProblem{Path: path, Error: err.Error()})
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			res.Errors++
			res.Problems = append(res.Problems, ScanProblem{Path: path, Error: err.Error()})
			return nil
		}
		rel = filepath.ToSlash(rel)
		if prev, ok := states[rel]; ok && prev.MTimeNS == info.ModTime().UnixNano() && prev.Size == info.Size() && prev.Status != "error" {
			res.Skipped++
			return nil
		}

		sha, err := sha256File(path)
		if err != nil {
			s.recordProblem(ctx, rel, info, "", "error", err, &res)
			return nil
		}
		if existing, err := s.Store.DocumentBySHA(ctx, sha); err == nil {
			res.Duplicates++
			state := store.LibraryFileState{
				Path: rel, MTimeNS: info.ModTime().UnixNano(), Size: info.Size(),
				SHA256: sha, DocumentID: existing.ID, Status: "duplicate",
			}
			_ = s.Store.UpsertLibraryFile(ctx, state, "")
			return nil
		} else if !errors.Is(err, store.ErrNotFound) {
			s.recordProblem(ctx, rel, info, sha, "error", err, &res)
			return nil
		}

		text, ocrStatus, extractErr := ExtractFile(ctx, path, lang)
		if extractErr != nil {
			ocrStatus = OCRError
		}
		c := Classify(rel, text, taxpayers, active, info.ModTime())
		if extractErr != nil {
			c.Review = true
			c.Reasons = append(c.Reasons, "no se pudo extraer el texto: "+extractErr.Error())
		}

		absDest := filepath.Join(root, filepath.FromSlash(Destination(root, taxpayerFor(c, taxpayers), c.Year, c.Period, c.Kind, name)))
		absDest, err = UniquePath(absDest, sha)
		if err != nil {
			s.recordProblem(ctx, rel, info, sha, "error", err, &res)
			return nil
		}
		copied := false
		if filepath.Clean(absDest) != resolved {
			if err := copyFile(path, absDest); err != nil {
				s.recordProblem(ctx, rel, info, sha, "error", err, &res)
				return nil
			}
			copied = true
		}
		libRel, err := filepath.Rel(root, absDest)
		if err != nil {
			s.recordProblem(ctx, rel, info, sha, "error", err, &res)
			return nil
		}
		libRel = filepath.ToSlash(libRel)

		doc := domain.Document{
			TaxpayerID:      c.TaxpayerID,
			Kind:            c.Kind,
			Filename:        filepath.Base(absDest),
			MIME:            mimeFor(absDest),
			Size:            info.Size(),
			SHA256:          sha,
			StoragePath:     libRel,
			StorageRoot:     root,
			LibraryPath:     libRel,
			SourcePath:      path,
			DocDate:         c.DocDate,
			AmountCents:     c.AmountCents,
			CounterpartyNIF: c.CounterpartyNIF,
			TextContent:     text,
			OCRStatus:       ocrStatus,
			ReviewStatus:    "ok",
			Metadata: map[string]any{
				"original_name": name,
				"imported":      true,
				"model_code":    c.ModelCode,
				"period_label":  c.PeriodLabel,
				"reasons":       c.Reasons,
			},
		}
		if c.Review {
			doc.ReviewStatus = "review"
			res.NeedsReview++
		}
		if c.TaxpayerID != nil && c.ModelCode != "" && c.PeriodLabel != "" {
			if filingID, err := s.Store.FindFilingByModelPeriod(ctx, *c.TaxpayerID, c.ModelCode, c.PeriodLabel); err == nil {
				doc.FilingID = &filingID
			}
		}
		if err := s.Store.CreateDocument(ctx, &doc); err != nil {
			if copied {
				_ = os.Remove(absDest)
			}
			s.recordProblem(ctx, rel, info, sha, "error", err, &res)
			return nil
		}
		taxpayerID := ""
		if c.TaxpayerID != nil {
			taxpayerID = *c.TaxpayerID
		}
		_ = s.Store.UpsertLibraryFile(ctx, store.LibraryFileState{
			Path: rel, MTimeNS: info.ModTime().UnixNano(), Size: info.Size(),
			SHA256: sha, DocumentID: doc.ID, Status: "imported",
		}, taxpayerID)
		res.Imported++
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) && !errors.Is(walkErr, context.Canceled) {
		return res, walkErr
	}
	res.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if raw, err := json.Marshal(res); err == nil {
		_ = s.Store.SetSetting(ctx, SettingLastScan, string(raw))
	}
	s.Store.Log(ctx, "web", "scan", "library", root, res)
	return res, nil
}

func (s *Service) recordProblem(ctx context.Context, rel string, info fs.FileInfo, sha, status string, err error, res *ScanResult) {
	res.Errors++
	res.Problems = append(res.Problems, ScanProblem{Path: rel, Error: err.Error()})
	_ = s.Store.UpsertLibraryFile(ctx, store.LibraryFileState{
		Path: rel, MTimeNS: info.ModTime().UnixNano(), Size: info.Size(),
		SHA256: sha, Status: status, Error: err.Error(),
	}, "")
}

func taxpayerFor(c Classification, taxpayers []domain.Taxpayer) domain.Taxpayer {
	if c.TaxpayerID != nil {
		for _, t := range taxpayers {
			if t.ID == *c.TaxpayerID {
				return t
			}
		}
	}
	return domain.Taxpayer{Name: "Sin clasificar", NIF: ""}
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".asesoria-copy-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, dst)
}

func mimeFor(path string) string {
	if m := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); m != "" {
		return m
	}
	return "application/octet-stream"
}

type UploadInput struct {
	Taxpayer        domain.Taxpayer
	FilingID        *string
	Kind            string
	Filename        string
	MIME            string
	DocDate         string
	AmountCents     *int64
	CounterpartyNIF string
	Reader          io.Reader
}

// Ingest stores an uploaded file inside the configured library tree, extracts
// its text and records the document.
func (s *Service) Ingest(ctx context.Context, in UploadInput) (domain.Document, error) {
	root, err := s.Root(ctx)
	if err != nil {
		return domain.Document{}, err
	}
	if root == "" {
		return domain.Document{}, ErrDisabled
	}
	if _, err := EnsureTaxpayerTree(root, in.Taxpayer, Years()); err != nil {
		return domain.Document{}, err
	}
	tmp, err := os.CreateTemp(root, ".asesoria-upload-*"+filepath.Ext(in.Filename))
	if err != nil {
		return domain.Document{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	h := sha256.New()
	const maxUpload = 64 << 20
	size, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(in.Reader, maxUpload+1))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return domain.Document{}, err
	}
	if size > maxUpload {
		return domain.Document{}, errors.New("el fichero supera el límite de 64 MB")
	}
	sha := hex.EncodeToString(h.Sum(nil))

	if existing, err := s.Store.DocumentBySHA(ctx, sha); err == nil {
		return existing, ErrDuplicate
	} else if !errors.Is(err, store.ErrNotFound) {
		return domain.Document{}, err
	}

	text, ocrStatus, _ := ExtractFile(ctx, tmpName, s.OCRLang(ctx))
	date := strings.TrimSpace(in.DocDate)
	if date == "" {
		date = dateFromText(text)
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	year, period := PeriodFromDate(date)
	kind := strings.TrimSpace(in.Kind)
	if kind == "" || kind == "other" {
		if fromName := kindFromFilename([]string{in.Filename}); fromName != "" {
			kind = fromName
		}
	}
	if kind == "" {
		kind = "other"
	}
	rel := Destination(root, in.Taxpayer, year, period, kind, in.Filename)
	absDest := filepath.Join(root, filepath.FromSlash(rel))
	absDest, err = UniquePath(absDest, sha)
	if err != nil {
		return domain.Document{}, err
	}
	if err := os.MkdirAll(filepath.Dir(absDest), 0o750); err != nil {
		return domain.Document{}, err
	}
	if err := os.Rename(tmpName, absDest); err != nil {
		if err := copyFile(tmpName, absDest); err != nil {
			return domain.Document{}, err
		}
	}
	libRel, _ := filepath.Rel(root, absDest)
	libRel = filepath.ToSlash(libRel)
	doc := domain.Document{
		TaxpayerID:      &in.Taxpayer.ID,
		FilingID:        in.FilingID,
		Kind:            kind,
		Filename:        filepath.Base(absDest),
		MIME:            in.MIME,
		Size:            size,
		SHA256:          sha,
		StoragePath:     libRel,
		StorageRoot:     root,
		LibraryPath:     libRel,
		SourcePath:      "subida",
		DocDate:         date,
		AmountCents:     in.AmountCents,
		CounterpartyNIF: strings.ToUpper(strings.TrimSpace(in.CounterpartyNIF)),
		TextContent:     text,
		OCRStatus:       ocrStatus,
		ReviewStatus:    "ok",
		Metadata: map[string]any{
			"original_name": in.Filename,
			"imported":      false,
		},
	}
	if doc.MIME == "" {
		doc.MIME = mimeFor(absDest)
	}
	if err := s.Store.CreateDocument(ctx, &doc); err != nil {
		_ = os.Remove(absDest)
		return domain.Document{}, err
	}
	return doc, nil
}
