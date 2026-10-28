package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

func (s *Store) CreateDocument(ctx context.Context, d *domain.Document) error {
	if d.ID == "" {
		d.ID = NewID()
	}
	d.CreatedAt = Now()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO documents
		(id, taxpayer_id, filing_id, kind, filename, mime, size, sha256, storage_path,
		 doc_date, amount_cents, counterparty_nif, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, nullStringPtr(d.TaxpayerID), nullStringPtr(d.FilingID), d.Kind, d.Filename, d.MIME,
		d.Size, d.SHA256, d.StoragePath, d.DocDate, nullInt64Ptr(d.AmountCents),
		d.CounterpartyNIF, jsonString(d.Metadata), d.CreatedAt)
	return err
}

func (s *Store) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, taxpayer_id, filing_id, kind, filename, mime,
		size, sha256, storage_path, doc_date, amount_cents, counterparty_nif, metadata_json, created_at
		FROM documents WHERE id = ?`, id)
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
	query := `SELECT id, taxpayer_id, filing_id, kind, filename, mime, size, sha256,
		storage_path, doc_date, amount_cents, counterparty_nif, metadata_json, created_at
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

func scanDocument(row scanner) (domain.Document, error) {
	var d domain.Document
	var taxpayerID, filingID, docDate, counterparty sql.NullString
	var amount sql.NullInt64
	var metadata string
	err := row.Scan(&d.ID, &taxpayerID, &filingID, &d.Kind, &d.Filename, &d.MIME, &d.Size,
		&d.SHA256, &d.StoragePath, &docDate, &amount, &counterparty, &metadata, &d.CreatedAt)
	if err != nil {
		return d, err
	}
	if taxpayerID.Valid {
		d.TaxpayerID = &taxpayerID.String
	}
	if filingID.Valid {
		d.FilingID = &filingID.String
	}
	d.DocDate = docDate.String
	d.CounterpartyNIF = counterparty.String
	if amount.Valid {
		v := amount.Int64
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
