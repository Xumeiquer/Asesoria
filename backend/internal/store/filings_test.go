package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Xumeiquer/asesoria/internal/db"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
)

func TestPreviousFilingsUsesFiscalYearAndPeriod(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	st := store.New(conn)
	ctx := context.Background()
	taxpayer := domain.Taxpayer{
		NIF: "12345678Z", Name: "Test", LegalForm: domain.LegalFormAutonomo,
		VATRegime: "general", IRPFMethod: "directa",
	}
	if err := st.CreateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}

	periods := []struct {
		label string
		start string
		end   string
		due   string
	}{
		{"4T 2025", "2025-10-01", "2025-12-31", "2026-01-20"},
		{"1T 2026", "2026-01-01", "2026-03-31", "2026-04-20"},
		{"2T 2026", "2026-04-01", "2026-06-30", "2026-07-20"},
		{"3T 2026", "2026-07-01", "2026-09-30", "2026-10-20"},
		{"4T 2026", "2026-10-01", "2026-12-31", "2027-01-20"},
	}
	filings := make([]domain.Filing, 0, len(periods))
	for _, p := range periods {
		filings = append(filings, domain.Filing{
			TaxpayerID: taxpayer.ID, ModelCode: "130", ModelName: "Pago fraccionado",
			Entity: "AEAT", PeriodLabel: p.label, PeriodStart: p.start, PeriodEnd: p.end,
			DueDate: p.due, Status: domain.StatusPending,
		})
	}
	if _, err := st.UpsertFilings(ctx, filings); err != nil {
		t.Fatal(err)
	}

	list, err := st.ListFilings(ctx, store.FilingFilter{TaxpayerID: taxpayer.ID, ModelCode: "130"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range list {
		list[i].Status = domain.StatusReady
		list[i].Computed = map[string]any{"values": map[string]string{"07": "10.00"}}
		if err := st.UpdateFiling(ctx, &list[i]); err != nil {
			t.Fatal(err)
		}
	}

	previous, err := st.PreviousFilings(ctx, taxpayer.ID, "130", 2026, "2026-07-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(previous) != 2 {
		t.Fatalf("expected 2 previous filings, got %d", len(previous))
	}
	if previous[0].PeriodLabel != "1T 2026" || previous[1].PeriodLabel != "2T 2026" {
		t.Fatalf("unexpected periods: %s, %s", previous[0].PeriodLabel, previous[1].PeriodLabel)
	}
}
