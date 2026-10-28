package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

type FilingFilter struct {
	TaxpayerID string
	Year       int
	From       string
	To         string
	Statuses   []string
	ModelCode  string
	Limit      int
}

func (s *Store) ListFilings(ctx context.Context, f FilingFilter) ([]domain.Filing, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if f.TaxpayerID != "" {
		where = append(where, "taxpayer_id = ?")
		args = append(args, f.TaxpayerID)
	}
	if f.Year > 0 {
		where = append(where, "due_date >= ? AND due_date <= ?")
		args = append(args, fmt.Sprintf("%d-01-01", f.Year), fmt.Sprintf("%d-12-31", f.Year))
	}
	if f.From != "" {
		where = append(where, "due_date >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		where = append(where, "due_date <= ?")
		args = append(args, f.To)
	}
	if f.ModelCode != "" {
		where = append(where, "model_code = ?")
		args = append(args, f.ModelCode)
	}
	if len(f.Statuses) > 0 {
		placeholders := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			placeholders[i] = "?"
			args = append(args, st)
		}
		where = append(where, "status IN ("+strings.Join(placeholders, ",")+")")
	}
	query := `SELECT id, taxpayer_id, model_code, model_name, entity, period_label,
		period_start, period_end, due_date, status, data_json, computed_json, notes,
		presented_at, receipt_document_id, created_at, updated_at
		FROM filings WHERE ` + strings.Join(where, " AND ") + ` ORDER BY due_date ASC, model_code ASC`
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Filing, 0)
	for rows.Next() {
		filing, err := scanFiling(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, filing)
	}
	return out, rows.Err()
}

func (s *Store) GetFiling(ctx context.Context, id string) (domain.Filing, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, taxpayer_id, model_code, model_name, entity,
		period_label, period_start, period_end, due_date, status, data_json, computed_json,
		notes, presented_at, receipt_document_id, created_at, updated_at
		FROM filings WHERE id = ?`, id)
	f, err := scanFiling(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Filing{}, ErrNotFound
	}
	return f, err
}

// UpsertFilings inserts generated calendar entries, ignoring periods that already exist.
func (s *Store) UpsertFilings(ctx context.Context, filings []domain.Filing) (int, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO filings
		(id, taxpayer_id, model_code, model_name, entity, period_label, period_start, period_end,
		 due_date, status, data_json, computed_json, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '{}', '{}', '', ?, ?)
		ON CONFLICT(taxpayer_id, model_code, period_label) DO NOTHING`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	now := Now()
	for _, f := range filings {
		res, err := stmt.ExecContext(ctx, NewID(), f.TaxpayerID, f.ModelCode, f.ModelName, f.Entity,
			f.PeriodLabel, f.PeriodStart, f.PeriodEnd, f.DueDate, domain.StatusPending, now, now)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			inserted++
		}
	}
	return inserted, tx.Commit()
}

func (s *Store) UpdateFiling(ctx context.Context, f *domain.Filing) error {
	f.UpdatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE filings SET
		status = ?, data_json = ?, computed_json = ?, notes = ?, updated_at = ?
		WHERE id = ?`,
		f.Status, jsonString(f.Data), jsonString(f.Computed), f.Notes, f.UpdatedAt, f.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkPresented(ctx context.Context, id, receiptDocumentID string) error {
	now := Now()
	var receipt any
	if receiptDocumentID != "" {
		receipt = receiptDocumentID
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE filings
		SET status = 'presented', presented_at = ?, receipt_document_id = ?, updated_at = ?
		WHERE id = ?`, now, receipt, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetFilingStatus(ctx context.Context, id, status string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE filings SET status = ?, updated_at = ? WHERE id = ?`, status, Now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) PreviousFilings(ctx context.Context, taxpayerID, modelCode string, year int, beforePeriodStart string) ([]domain.Filing, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, taxpayer_id, model_code, model_name, entity,
		period_label, period_start, period_end, due_date, status, data_json, computed_json,
		notes, presented_at, receipt_document_id, created_at, updated_at
		FROM filings
		WHERE taxpayer_id = ? AND model_code = ? AND period_start >= ? AND period_start < ?
		  AND status IN ('ready','presented')
		ORDER BY period_start ASC`,
		taxpayerID, modelCode, fmt.Sprintf("%d-01-01", year), beforePeriodStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Filing, 0)
	for rows.Next() {
		f, err := scanFiling(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func scanFiling(row scanner) (domain.Filing, error) {
	var f domain.Filing
	var dataJSON, computedJSON string
	var presentedAt, receipt sql.NullString
	err := row.Scan(&f.ID, &f.TaxpayerID, &f.ModelCode, &f.ModelName, &f.Entity,
		&f.PeriodLabel, &f.PeriodStart, &f.PeriodEnd, &f.DueDate, &f.Status,
		&dataJSON, &computedJSON, &f.Notes, &presentedAt, &receipt, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return f, err
	}
	f.Data = decodeMap(dataJSON)
	f.Computed = decodeMap(computedJSON)
	if presentedAt.Valid {
		f.PresentedAt = &presentedAt.String
	}
	if receipt.Valid {
		f.ReceiptDocumentID = &receipt.String
	}
	return f, nil
}
