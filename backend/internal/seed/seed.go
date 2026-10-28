// Package seed creates optional demo data for local development.
package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/Xumeiquer/asesoria/internal/calc"
	"github.com/Xumeiquer/asesoria/internal/calendar"
	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
)

// Demo creates an autónomo and an SL with invoices and a ready-to-review 303.
// It is a no-op when the database already has taxpayers.
func Demo(ctx context.Context, st *store.Store, cat *catalog.Catalog) error {
	existing, err := st.ListTaxpayers(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	gen := calendar.New(cat)
	now := time.Now()

	taxpayers := []domain.Taxpayer{
		{
			NIF: "12345678Z", Name: "Lucía Demo", LegalForm: domain.LegalFormAutonomo,
			Email: "lucia@example.test", VATRegime: "general", IRPFMethod: "directa",
			EconomicActivity: true, CNAE: "7022", FiscalYearEnd: "12-31",
		},
		{
			NIF: "B12345678", Name: "Estudio Demo SL", LegalForm: domain.LegalFormSL,
			Email: "admin@estudiodemo.test", VATRegime: "general", IRPFMethod: "directa",
			HasEmployees: true, HasRentals: true, EconomicActivity: true,
			CNAE: "6201", FiscalYearEnd: "12-31",
		},
	}
	for i := range taxpayers {
		if err := st.CreateTaxpayer(ctx, &taxpayers[i]); err != nil {
			return fmt.Errorf("create demo taxpayer: %w", err)
		}
		for _, year := range []int{now.Year(), now.Year() + 1} {
			if _, err := st.UpsertFilings(ctx, gen.Generate(taxpayers[i], year)); err != nil {
				return err
			}
		}
	}

	// Invoices inside the current quarter.
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for start.Month()%3 != 1 {
		start = start.AddDate(0, -1, 0)
	}
	end := start.AddDate(0, 3, -1)
	mid := start.AddDate(0, 0, 4)

	invoices := []struct {
		idx       int
		direction string
		number    string
		date      time.Time
		nif       string
		name      string
		concept   string
		base      int64
		rate      int
		retention int
	}{
		{0, domain.DirectionIssued, "F-2026-001", mid, "B87654321", "Cliente Uno SL", "Servicios de consultoría", 120000, 2100, 0},
		{0, domain.DirectionIssued, "F-2026-002", mid.AddDate(0, 0, 2), "B87654321", "Cliente Dos SL", "Diseño de producto", 85000, 2100, 1500},
		{0, domain.DirectionIssued, "F-2026-003", mid.AddDate(0, 0, 5), "12345678Z", "Particular", "Formación", 40000, 1000, 0},
		{0, domain.DirectionReceived, "G-001", mid.AddDate(0, 0, 1), "B11111111", "Proveedor Cloud SL", "Hosting", 30000, 2100, 0},
		{0, domain.DirectionReceived, "G-002", mid.AddDate(0, 0, 3), "B22222222", "Coworking SL", "Alquiler de despacho", 45000, 2100, 1900},
		{1, domain.DirectionIssued, "A-2026-001", mid, "B87654321", "Cliente Uno SL", "Auditoría", 250000, 2100, 0},
		{1, domain.DirectionIssued, "A-2026-002", mid.AddDate(0, 0, 4), "B33333333", "Cliente Tres SL", "Consultoría", 180000, 2100, 0},
		{1, domain.DirectionReceived, "C-001", mid.AddDate(0, 0, 2), "B44444444", "Oficina SL", "Alquiler", 90000, 2100, 1900},
		{1, domain.DirectionReceived, "C-002", mid.AddDate(0, 0, 6), "B55555555", "Material SL", "Equipos", 120000, 2100, 0},
	}
	for _, item := range invoices {
		vat := (item.base*int64(item.rate) + 5000) / 10000
		ret := (item.base*int64(item.retention) + 5000) / 10000
		inv := domain.Invoice{
			TaxpayerID:       taxpayers[item.idx].ID,
			Direction:        item.direction,
			Number:           item.number,
			InvoiceDate:      item.date.Format("2006-01-02"),
			CounterpartyNIF:  item.nif,
			CounterpartyName: item.name,
			Concept:          item.concept,
			BaseCents:        item.base,
			VATRate:          item.rate,
			VATCents:         vat,
			RetentionRate:    item.retention,
			RetentionCents:   ret,
			TotalCents:       item.base + vat - ret,
		}
		if err := st.CreateInvoice(ctx, &inv); err != nil {
			return fmt.Errorf("create demo invoice: %w", err)
		}
	}

	// Leave the current quarter's 303 ready for review.
	for _, year := range []int{now.Year(), now.Year() + 1} {
		filings, err := st.ListFilings(ctx, store.FilingFilter{TaxpayerID: taxpayers[0].ID, Year: year, ModelCode: "303"})
		if err != nil {
			return err
		}
		done := false
		for _, f := range filings {
			if f.PeriodEnd < start.Format("2006-01-02") || f.PeriodStart > end.Format("2006-01-02") {
				continue
			}
			invs, err := st.InvoicesForPeriod(ctx, taxpayers[0].ID, f.PeriodStart, f.PeriodEnd)
			if err != nil {
				return err
			}
			if computed, ok := calc.Calculate("303", invs, 0); ok {
				f.Computed = computed
				f.Status = domain.StatusReady
				if err := st.UpdateFiling(ctx, &f); err != nil {
					return err
				}
			}
			done = true
			break
		}
		if done {
			break
		}
	}
	return nil
}
