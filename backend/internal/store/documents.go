package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

const documentColumns = `id, taxpayer_id, filing_id, kind, filename, mime, size, sha256,
	storage_path, storage_root, library_path, source_path, doc_date, amount_cents,
	counterparty_nif, substr(text_content, 1, 600), ocr_status, review_status,
	metadata_json, created_at`

func (s *Store) CreateDocument(ctx context.Context, d *domain.Document) error {
	if d.ID == "" {
		d.ID = NewID()
	}
	d.CreatedAt = Now()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO documents
		(id, taxpayer_id, filing_id, kind, filename, mime, size, sha256, storage_path,
		 storage_root, library_path, source_path, doc_date, amount_cents, counterparty_nif,
		 text_content, ocr_status, review_status, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, nullStringPtr(d.TaxpayerID), nullStringPtr(d.FilingID), d.Kind, d.Filename, d.MIME,
		d.Size, d.SHA256, d.StoragePath, d.StorageRoot, d.LibraryPath, d.SourcePath, d.DocDate,
		nullInt64Ptr(d.AmountCents), d.CounterpartyNIF, d.TextContent, d.OCRStatus,
		d.ReviewStatus, jsonString(d.Metadata), d.CreatedAt)
	return err
}

func (s *Store) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+documentColumns+` FROM documents WHERE id = ?`, id)
	d, err := scanDocument(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Document{}, ErrNotFound
	}
	return d, err
}

func (s *Store) DocumentBySHA(ctx context.Context, sha string) (domain.Document, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+documentColumns+` FROM documents
		WHERE sha256 = ? ORDER BY created_at DESC LIMIT 1`, sha)
	d, err := scanDocument(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Document{}, ErrNotFound
	}
	return d, err
}

type DocumentFilter struct {
	TaxpayerID string
	FilingID   string
	Kind       string
	Review     string
	Year       int
	Quarter    int
	Query      string
	Limit      int
}

func (s *Store) ListDocuments(ctx context.Context, f DocumentFilter) ([]domain.Document, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if f.TaxpayerID != "" {
		where = append(where, "taxpayer_id = ?")
		args = append(args, f.TaxpayerID)
	}
	if f.FilingID != "" {
		where = append(where, "filing_id = ?")
		args = append(args, f.FilingID)
	}
	if f.Kind != "" {
		where = append(where, "kind = ?")
		args = append(args, f.Kind)
	}
	if f.Review != "" {
		where = append(where, "review_status = ?")
		args = append(args, f.Review)
	}
	if f.Year > 0 {
		where = append(where, `((doc_date <> '' AND substr(doc_date, 1, 4) = ?) OR
			(doc_date = '' AND substr(created_at, 1, 4) = ?))`)
		args = append(args, itoa(f.Year), itoa(f.Year))
	}
	if f.Quarter > 0 {
		where = append(where, `((doc_date <> '' AND
			(CAST(substr(doc_date, 6, 2) AS INTEGER) + 2) / 3 = ?) OR
			(doc_date = '' AND (CAST(substr(created_at, 6, 2) AS INTEGER) + 2) / 3 = ?))`)
		args = append(args, f.Quarter, f.Quarter)
	}
	if q := f.Query; q != "" {
		like := "%" + q + "%"
		where = append(where, `(filename LIKE ? OR text_content LIKE ? OR
			counterparty_nif LIKE ? OR source_path LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	query := `SELECT ` + documentColumns + `
		FROM documents WHERE ` + joinAnd(where) + ` ORDER BY created_at DESC`
	if f.Limit > 0 {
		query += " LIMIT " + itoa(f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Document, 0)
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDocument(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateDocumentAnalysis(ctx context.Context, id, text, ocrStatus, reviewStatus string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE documents
		SET text_content = ?, ocr_status = ?, review_status = ? WHERE id = ?`,
		text, ocrStatus, reviewStatus, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type DocumentMetadata struct {
	TaxpayerID      *string
	FilingID        *string
	Kind            string
	DocDate         string
	AmountCents     *int64
	CounterpartyNIF string
	ReviewStatus    string
}

func (s *Store) UpdateDocumentMetadata(ctx context.Context, id string, in DocumentMetadata) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE documents
		SET taxpayer_id = ?, filing_id = ?, kind = ?, doc_date = ?, amount_cents = ?,
		    counterparty_nif = ?, review_status = ? WHERE id = ?`,
		nullStringPtr(in.TaxpayerID), nullStringPtr(in.FilingID), in.Kind, in.DocDate,
		nullInt64Ptr(in.AmountCents), in.CounterpartyNIF, in.ReviewStatus, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// LibraryPaths returns the relative library paths already registered, so scans
// can skip files that the ERP itself wrote.
func (s *Store) LibraryPaths(ctx context.Context) (map[string]bool, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT library_path FROM documents WHERE library_path <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out[p] = true
	}
	return out, rows.Err()
}

type LibraryFileState struct {
	Path       string
	MTimeNS    int64
	Size       int64
	SHA256     string
	DocumentID string
	Status     string
	Error      string
}

func (s *Store) LibraryFiles(ctx context.Context) (map[string]LibraryFileState, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT path, mtime_ns, size, sha256,
		COALESCE(document_id, ''), status, error FROM library_files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]LibraryFileState{}
	for rows.Next() {
		var f LibraryFileState
		if err := rows.Scan(&f.Path, &f.MTimeNS, &f.Size, &f.SHA256, &f.DocumentID, &f.Status, &f.Error); err != nil {
			return nil, err
		}
		out[f.Path] = f
	}
	return out, rows.Err()
}

func (s *Store) UpsertLibraryFile(ctx context.Context, f LibraryFileState, taxpayerID string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO library_files
		(path, taxpayer_id, mtime_ns, size, sha256, document_id, status, error, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			taxpayer_id = excluded.taxpayer_id, mtime_ns = excluded.mtime_ns,
			size = excluded.size, sha256 = excluded.sha256,
			document_id = excluded.document_id, status = excluded.status,
			error = excluded.error, scanned_at = excluded.scanned_at`,
		f.Path, nullStringPtr(&taxpayerID), f.MTimeNS, f.Size, f.SHA256,
		nullStringPtr(&f.DocumentID), f.Status, f.Error, Now())
	return err
}

func (s *Store) FindFilingByModelPeriod(ctx context.Context, taxpayerID, modelCode, periodLabel string) (string, error) {
	var id string
	err := s.DB.QueryRowContext(ctx, `SELECT id FROM filings
		WHERE taxpayer_id = ? AND model_code = ? AND period_label = ?`,
		taxpayerID, modelCode, periodLabel).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func scanDocument(row scanner) (domain.Document, error) {
	var d domain.Document
	var amountCents sql.NullInt64
	var tID, fID sql.NullString
	var metadata string
	err := row.Scan(&d.ID, &tID, &fID, &d.Kind, &d.Filename, &d.MIME, &d.Size,
		&d.SHA256, &d.StoragePath, &d.StorageRoot, &d.LibraryPath, &d.SourcePath,
		&d.DocDate, &amountCents, &d.CounterpartyNIF, &d.TextExcerpt, &d.OCRStatus,
		&d.ReviewStatus, &metadata, &d.CreatedAt)
	if err != nil {
		return d, err
	}
	if tID.Valid {
		d.TaxpayerID = &tID.String
	}
	if fID.Valid {
		d.FilingID = &fID.String
	}
	if amountCents.Valid {
		v := amountCents.Int64
		d.AmountCents = &v
	}
	d.Metadata = decodeMap(metadata)
	return d, nil
}

func nullStringPtr(v *string) any {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}

func nullInt64Ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
