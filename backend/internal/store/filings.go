package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
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
		period_start, period_end, due_date, status, auto_hidden, data_json, computed_json, notes,
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
		period_label, period_start, period_end, due_date, status, auto_hidden, data_json, computed_json,
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

// FilingSyncResult summarises a calendar recalculation.
type FilingSyncResult struct {
	Inserted      int `json:"inserted"`
	Updated       int `json:"updated"`
	Hidden        int `json:"hidden"`
	Restored      int `json:"restored"`
	KeptPresented int `json:"kept_presented"`
}

// SyncFilings reconciles the calendar for the given due-date years with the
// generated entries. Missing filings are inserted, existing metadata and dates
// are refreshed, and filings that no longer apply are retired as
// not_applicable. Presented filings are never modified or hidden, and manual
// not_applicable decisions are preserved. Filings retired automatically are
// restored when they become applicable again.
func (s *Store) SyncFilings(ctx context.Context, taxpayerID string, generated []domain.Filing, years []int) (FilingSyncResult, error) {
	result := FilingSyncResult{}
	yearSet := make(map[int]struct{}, len(years))
	for _, year := range years {
		if year > 0 {
			yearSet[year] = struct{}{}
		}
	}
	for _, f := range generated {
		if year := dueYear(f.DueDate); year > 0 {
			yearSet[year] = struct{}{}
		}
	}
	if len(yearSet) == 0 {
		return result, nil
	}

	yearValues := make([]int, 0, len(yearSet))
	for year := range yearSet {
		yearValues = append(yearValues, year)
	}
	sort.Ints(yearValues)
	placeholders := make([]string, len(yearValues))
	args := make([]any, 0, len(yearValues)+1)
	args = append(args, taxpayerID)
	for i, year := range yearValues {
		placeholders[i] = "?"
		args = append(args, year)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	query := `SELECT id, taxpayer_id, model_code, model_name, entity, period_label,
		period_start, period_end, due_date, status, auto_hidden, data_json, computed_json, notes,
		presented_at, receipt_document_id, created_at, updated_at
		FROM filings
		WHERE taxpayer_id = ? AND CAST(substr(due_date, 1, 4) AS INTEGER) IN (` +
		strings.Join(placeholders, ",") + `)`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	existing := make(map[string]domain.Filing)
	for rows.Next() {
		f, err := scanFiling(rows)
		if err != nil {
			rows.Close()
			return result, err
		}
		existing[filingKey(f)] = f
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()

	now := Now()
	seen := make(map[string]struct{}, len(generated))
	for _, f := range generated {
		if f.TaxpayerID == "" {
			f.TaxpayerID = taxpayerID
		}
		key := filingKey(f)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}

		old, exists := existing[key]
		if !exists {
			status := f.Status
			if status == "" {
				status = domain.StatusPending
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO filings
				(id, taxpayer_id, model_code, model_name, entity, period_label, period_start, period_end,
				 due_date, status, data_json, computed_json, notes, auto_hidden, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '{}', '{}', '', 0, ?, ?)`,
				NewID(), f.TaxpayerID, f.ModelCode, f.ModelName, f.Entity, f.PeriodLabel,
				f.PeriodStart, f.PeriodEnd, f.DueDate, status, now, now); err != nil {
				return result, err
			}
			result.Inserted++
			continue
		}

		nextStatus := old.Status
		nextAutoHidden := old.AutoHidden
		restored := false
		if old.AutoHidden {
			nextAutoHidden = false
			if old.Status == domain.StatusNotApplicable {
				nextStatus = domain.StatusPending
				restored = true
			}
		}
		changed := restored ||
			nextAutoHidden != old.AutoHidden ||
			old.ModelName != f.ModelName ||
			old.Entity != f.Entity ||
			old.PeriodStart != f.PeriodStart ||
			old.PeriodEnd != f.PeriodEnd ||
			old.DueDate != f.DueDate
		if !changed {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE filings SET
			model_name = ?, entity = ?, period_start = ?, period_end = ?, due_date = ?,
			status = ?, auto_hidden = ?, updated_at = ?
			WHERE id = ?`,
			f.ModelName, f.Entity, f.PeriodStart, f.PeriodEnd, f.DueDate,
			nextStatus, boolInt(nextAutoHidden), now, old.ID); err != nil {
			return result, err
		}
		result.Updated++
		if restored {
			result.Restored++
		}
	}

	for key, old := range existing {
		if _, ok := seen[key]; ok {
			continue
		}
		if old.Status == domain.StatusPresented {
			result.KeptPresented++
			continue
		}
		if old.Status == domain.StatusNotApplicable {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE filings SET
			status = ?, auto_hidden = 1, updated_at = ?
			WHERE id = ?`, domain.StatusNotApplicable, now, old.ID); err != nil {
			return result, err
		}
		result.Hidden++
	}

	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func filingKey(f domain.Filing) string {
	return f.ModelCode + "\x00" + f.PeriodLabel
}

func dueYear(value string) int {
	if len(value) < 4 {
		return 0
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return 0
	}
	return year
}

func (s *Store) UpdateFiling(ctx context.Context, f *domain.Filing) error {
	f.UpdatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE filings SET
		status = ?, auto_hidden = ?, data_json = ?, computed_json = ?, notes = ?,
		presented_at = ?, receipt_document_id = ?, updated_at = ?
		WHERE id = ?`,
		f.Status, boolInt(f.AutoHidden), jsonString(f.Data), jsonString(f.Computed), f.Notes,
		nullStringPtr(f.PresentedAt), nullStringPtr(f.ReceiptDocumentID), f.UpdatedAt, f.ID)
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
		SET status = 'presented', auto_hidden = 0, presented_at = ?, receipt_document_id = ?, updated_at = ?
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
	res, err := s.DB.ExecContext(ctx, `UPDATE filings SET status = ?, auto_hidden = 0, updated_at = ? WHERE id = ?`, status, Now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// BulkSetFilingStatus applies the same state to several filings in one
// transaction. Marking as presented in bulk deliberately creates a presented
// filing without a receipt; the dashboard and calendar expose that difference.
func (s *Store) BulkSetFilingStatus(ctx context.Context, ids []string, status string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = "?"
	}
	now := Now()
	query := `UPDATE filings SET status = ?, auto_hidden = 0`
	args := []any{status}
	if status == domain.StatusPresented {
		query += `, presented_at = ?, receipt_document_id = NULL`
		args = append(args, now)
	} else {
		query += `, presented_at = NULL, receipt_document_id = NULL`
	}
	query += `, updated_at = ? WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	args = append(args, now)
	args = append(args, anyIDs(ids)...)
	res, err := s.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func anyIDs(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func (s *Store) PreviousFilings(ctx context.Context, taxpayerID, modelCode string, year int, beforePeriodStart string) ([]domain.Filing, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, taxpayer_id, model_code, model_name, entity,
		period_label, period_start, period_end, due_date, status, auto_hidden, data_json, computed_json,
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
	var autoHidden int
	var dataJSON, computedJSON string
	var presentedAt, receipt sql.NullString
	err := row.Scan(&f.ID, &f.TaxpayerID, &f.ModelCode, &f.ModelName, &f.Entity,
		&f.PeriodLabel, &f.PeriodStart, &f.PeriodEnd, &f.DueDate, &f.Status, &autoHidden,
		&dataJSON, &computedJSON, &f.Notes, &presentedAt, &receipt, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return f, err
	}
	f.AutoHidden = autoHidden != 0
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
