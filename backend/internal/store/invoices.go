package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

func (s *Store) CreateInvoice(ctx context.Context, inv *domain.Invoice) error {
	if inv.ID == "" {
		inv.ID = NewID()
	}
	inv.CreatedAt = Now()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO invoices
		(id, taxpayer_id, direction, number, invoice_date, counterparty_nif, counterparty_name,
		 concept, base_cents, vat_rate, vat_cents, retention_rate, retention_cents, total_cents,
		 category, document_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.TaxpayerID, inv.Direction, inv.Number, inv.InvoiceDate, inv.CounterpartyNIF,
		inv.CounterpartyName, inv.Concept, inv.BaseCents, inv.VATRate, inv.VATCents,
		inv.RetentionRate, inv.RetentionCents, inv.TotalCents, inv.Category,
		nullStringPtr(inv.DocumentID), inv.CreatedAt)
	return err
}

func (s *Store) GetInvoice(ctx context.Context, id string) (domain.Invoice, error) {
	row := s.DB.QueryRowContext(ctx, selectInvoice+` WHERE id = ?`, id)
	inv, err := scanInvoice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Invoice{}, ErrNotFound
	}
	return inv, err
}

type InvoiceFilter struct {
	TaxpayerID string
	Direction  string
	Year       int
	Limit      int
}

const selectInvoice = `SELECT id, taxpayer_id, direction, number, invoice_date, counterparty_nif,
	counterparty_name, concept, base_cents, vat_rate, vat_cents, retention_rate, retention_cents,
	total_cents, category, document_id, created_at FROM invoices`

func (s *Store) ListInvoices(ctx context.Context, f InvoiceFilter) ([]domain.Invoice, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if f.TaxpayerID != "" {
		where = append(where, "taxpayer_id = ?")
		args = append(args, f.TaxpayerID)
	}
	if f.Direction != "" {
		where = append(where, "direction = ?")
		args = append(args, f.Direction)
	}
	if f.Year > 0 {
		where = append(where, "invoice_date >= ? AND invoice_date <= ?")
		args = append(args, fmt.Sprintf("%d-01-01", f.Year), fmt.Sprintf("%d-12-31", f.Year))
	}
	query := selectInvoice + ` WHERE ` + strings.Join(where, " AND ") + ` ORDER BY invoice_date DESC, created_at DESC`
	if f.Limit > 0 {
		query += " LIMIT " + itoa(f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Invoice, 0)
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Store) InvoicesForPeriod(ctx context.Context, taxpayerID, start, end string) ([]domain.Invoice, error) {
	rows, err := s.DB.QueryContext(ctx, selectInvoice+`
		WHERE taxpayer_id = ? AND invoice_date >= ? AND invoice_date <= ?
		ORDER BY invoice_date ASC`, taxpayerID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Invoice, 0)
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Store) DeleteInvoice(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM invoices WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanInvoice(row scanner) (domain.Invoice, error) {
	var inv domain.Invoice
	var documentID sql.NullString
	err := row.Scan(&inv.ID, &inv.TaxpayerID, &inv.Direction, &inv.Number, &inv.InvoiceDate,
		&inv.CounterpartyNIF, &inv.CounterpartyName, &inv.Concept, &inv.BaseCents, &inv.VATRate,
		&inv.VATCents, &inv.RetentionRate, &inv.RetentionCents, &inv.TotalCents, &inv.Category,
		&documentID, &inv.CreatedAt)
	if documentID.Valid {
		inv.DocumentID = &documentID.String
	}
	return inv, err
}
