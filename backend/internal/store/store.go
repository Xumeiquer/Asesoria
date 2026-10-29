// Package store contains all SQL access. Queries are written by hand to keep
// the binary small and the SQL easy to audit.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store { return &Store{DB: db} }

const dateLayout = "2006-01-02"

func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// NewID returns a random 128-bit identifier, hex encoded.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func decodeMap(s string) map[string]any {
	m := map[string]any{}
	if s == "" {
		return m
	}
	_ = json.Unmarshal([]byte(s), &m)
	return m
}

func decodeStringSlice(s string) []string {
	values := []string{}
	if s == "" {
		return values
	}
	_ = json.Unmarshal([]byte(s), &values)
	if values == nil {
		values = []string{}
	}
	return values
}

func firstCNAE(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

type scanner interface {
	Scan(dest ...any) error
}

// --- settings ---------------------------------------------------------------

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, Now())
	return err
}

// EnsureSessionSecret returns a stable secret used to sign the local session cookie.
func (s *Store) EnsureSessionSecret(ctx context.Context) (string, error) {
	value, err := s.GetSetting(ctx, "session_secret")
	if err == nil && len(value) >= 32 {
		return value, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	value = hex.EncodeToString(b)
	return value, s.SetSetting(ctx, "session_secret", value)
}

// --- audit ------------------------------------------------------------------

func (s *Store) Log(ctx context.Context, actor, action, entity, entityID string, payload any) {
	_, _ = s.DB.ExecContext(ctx, `
		INSERT INTO audit_log (at, actor, action, entity, entity_id, payload_json)
		VALUES (?, ?, ?, ?, ?, ?)`,
		Now(), actor, action, entity, entityID, jsonString(payload))
}

// --- taxpayers --------------------------------------------------------------

func (s *Store) ListTaxpayers(ctx context.Context) ([]domain.Taxpayer, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, nif, name, legal_form, email, phone, vat_regime, irpf_method,
		       has_employees, has_rentals, economic_activity, cnaes_json, fiscal_year_end, iban,
		       activity_start,
		       reta_base_cents, reta_base_from, reta_net_income_cents, reta_net_income_year,
		       created_at, updated_at
		FROM taxpayers ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Taxpayer, 0)
	for rows.Next() {
		t, err := scanTaxpayer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTaxpayer(ctx context.Context, id string) (domain.Taxpayer, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, nif, name, legal_form, email, phone, vat_regime, irpf_method,
		       has_employees, has_rentals, economic_activity, cnaes_json, fiscal_year_end, iban,
		       activity_start,
		       reta_base_cents, reta_base_from, reta_net_income_cents, reta_net_income_year,
		       created_at, updated_at
		FROM taxpayers WHERE id = ?`, id)
	t, err := scanTaxpayer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Taxpayer{}, ErrNotFound
	}
	return t, err
}

func (s *Store) CreateTaxpayer(ctx context.Context, t *domain.Taxpayer) error {
	if t.ID == "" {
		t.ID = NewID()
	}
	t.CreatedAt = Now()
	t.UpdatedAt = t.CreatedAt
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO taxpayers (id, nif, name, legal_form, email, phone, vat_regime, irpf_method,
		                       has_employees, has_rentals, economic_activity, cnae, cnaes_json,
		                       fiscal_year_end, iban, activity_start,
		                       reta_base_cents, reta_base_from, reta_net_income_cents, reta_net_income_year,
		                       created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.NIF, t.Name, t.LegalForm, t.Email, t.Phone, t.VATRegime, t.IRPFMethod,
		boolInt(t.HasEmployees), boolInt(t.HasRentals), boolInt(t.EconomicActivity),
		firstCNAE(t.CNAEs), jsonString(t.CNAEs), t.FiscalYearEnd, t.IBAN, t.ActivityStart,
		t.RetaBaseCents, t.RetaBaseFrom, t.RetaNetIncomeCents, t.RetaNetIncomeYear,
		t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *Store) UpdateTaxpayer(ctx context.Context, t *domain.Taxpayer) error {
	t.UpdatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `
		UPDATE taxpayers SET nif = ?, name = ?, legal_form = ?, email = ?, phone = ?,
		       vat_regime = ?, irpf_method = ?, has_employees = ?, has_rentals = ?,
		       economic_activity = ?, cnae = ?, cnaes_json = ?, fiscal_year_end = ?, iban = ?,
		       activity_start = ?,
		       reta_base_cents = ?, reta_base_from = ?, reta_net_income_cents = ?, reta_net_income_year = ?,
		       updated_at = ?
		WHERE id = ?`,
		t.NIF, t.Name, t.LegalForm, t.Email, t.Phone, t.VATRegime, t.IRPFMethod,
		boolInt(t.HasEmployees), boolInt(t.HasRentals), boolInt(t.EconomicActivity),
		firstCNAE(t.CNAEs), jsonString(t.CNAEs), t.FiscalYearEnd, t.IBAN, t.ActivityStart,
		t.RetaBaseCents, t.RetaBaseFrom, t.RetaNetIncomeCents, t.RetaNetIncomeYear,
		t.UpdatedAt, t.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTaxpayer(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM taxpayers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddRetaBaseChange records an effective-dated snapshot of the current RETA
// profile. It is a no-op when the taxpayer has no chosen base.
func (s *Store) AddRetaBaseChange(ctx context.Context, t domain.Taxpayer) error {
	if t.ID == "" || t.RetaBaseCents == 0 {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO reta_base_history
		    (id, taxpayer_id, base_cents, base_from, net_income_cents, net_income_year, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		NewID(), t.ID, t.RetaBaseCents, t.RetaBaseFrom, t.RetaNetIncomeCents, t.RetaNetIncomeYear, Now())
	return err
}

func (s *Store) ListRetaBaseChanges(ctx context.Context, taxpayerID string) ([]domain.RetaBaseChange, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, taxpayer_id, base_cents, base_from, net_income_cents, net_income_year, created_at
		FROM reta_base_history
		WHERE taxpayer_id = ?
		ORDER BY base_from DESC, created_at DESC`, taxpayerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RetaBaseChange, 0)
	for rows.Next() {
		var change domain.RetaBaseChange
		if err := rows.Scan(&change.ID, &change.TaxpayerID, &change.BaseCents, &change.BaseFrom,
			&change.NetIncomeCents, &change.NetIncomeYear, &change.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, change)
	}
	return out, rows.Err()
}

func scanTaxpayer(row scanner) (domain.Taxpayer, error) {
	var t domain.Taxpayer
	var hasEmployees, hasRentals, economicActivity int
	var cnaesJSON string
	err := row.Scan(&t.ID, &t.NIF, &t.Name, &t.LegalForm, &t.Email, &t.Phone,
		&t.VATRegime, &t.IRPFMethod, &hasEmployees, &hasRentals, &economicActivity,
		&cnaesJSON, &t.FiscalYearEnd, &t.IBAN, &t.ActivityStart,
		&t.RetaBaseCents, &t.RetaBaseFrom, &t.RetaNetIncomeCents, &t.RetaNetIncomeYear,
		&t.CreatedAt, &t.UpdatedAt)
	t.HasEmployees = hasEmployees == 1
	t.HasRentals = hasRentals == 1
	t.EconomicActivity = economicActivity == 1
	t.CNAEs = decodeStringSlice(cnaesJSON)
	return t, err
}

// --- dashboard --------------------------------------------------------------

func (s *Store) Dashboard(ctx context.Context, taxpayerID string, year int) (domain.Dashboard, error) {
	var d domain.Dashboard
	d.ByEntity = map[string]int{}

	where := ""
	args := []any{}
	if taxpayerID != "" {
		where = " AND taxpayer_id = ?"
		args = append(args, taxpayerID)
	}

	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM taxpayers`).Scan(&d.Taxpayers)

	yearStart := fmt.Sprintf("%d-01-01", year)
	yearEnd := fmt.Sprintf("%d-12-31", year)
	today := time.Now().Format(dateLayout)
	in45 := time.Now().AddDate(0, 0, 45).Format(dateLayout)
	const openStatuses = "status IN ('pending','in_progress','ready')"

	// Pending and overdue are intentionally disjoint: pending means still in
	// term within the selected year, overdue means already past due within it.
	pendingArgs := append(append([]any{}, args...), today, yearEnd)
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM filings WHERE `+openStatuses+where+` AND due_date >= ? AND due_date <= ?`, pendingArgs...).Scan(&d.Pending); err != nil {
		return d, err
	}

	soonArgs := append(append([]any{}, args...), today, in45)
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM filings WHERE `+openStatuses+where+` AND due_date >= ? AND due_date <= ?`, soonArgs...).Scan(&d.DueSoon); err != nil {
		return d, err
	}

	overdueArgs := append(append([]any{}, args...), yearStart, today)
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM filings WHERE `+openStatuses+where+` AND due_date >= ? AND due_date < ?`, overdueArgs...).Scan(&d.Overdue); err != nil {
		return d, err
	}

	presentedArgs := append(append([]any{}, args...), yearStart, yearEnd)
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM filings WHERE status = 'presented'`+where+` AND due_date >= ? AND due_date <= ?`, presentedArgs...).Scan(&d.Presented); err != nil {
		return d, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM filings WHERE status = 'presented' AND receipt_document_id IS NOT NULL`+where+` AND due_date >= ? AND due_date <= ?`, presentedArgs...).Scan(&d.PresentedWithReceipt); err != nil {
		return d, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM filings WHERE status = 'presented' AND receipt_document_id IS NULL`+where+` AND due_date >= ? AND due_date <= ?`, presentedArgs...).Scan(&d.PresentedWithoutReceipt); err != nil {
		return d, err
	}

	docWhere := ""
	docArgs := []any{}
	if taxpayerID != "" {
		docWhere = " WHERE taxpayer_id = ?"
		docArgs = append(docArgs, taxpayerID)
	}
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`+docWhere, docArgs...).Scan(&d.Documents)
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices`+docWhere, docArgs...).Scan(&d.Invoices)

	rows, err := s.DB.QueryContext(ctx, `
		SELECT entity, COUNT(*) FROM filings
		WHERE status IN ('pending','in_progress','ready')`+where+`
		  AND due_date >= ? AND due_date <= ?
		GROUP BY entity`, append(append([]any{}, args...), yearStart, yearEnd)...)
	if err == nil {

		defer rows.Close()
		for rows.Next() {
			var entity string
			var n int
			if rows.Scan(&entity, &n) == nil {
				d.ByEntity[entity] = n
			}
		}
	}

	upcoming, err := s.ListFilings(ctx, FilingFilter{
		TaxpayerID: taxpayerID,
		From:       today,
		Statuses:   []string{domain.StatusPending, domain.StatusInProgress, domain.StatusReady},
		Limit:      8,
	})
	if err != nil {
		return d, err
	}
	d.Upcoming = upcoming
	return d, nil
}
