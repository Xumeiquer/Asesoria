package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Xumeiquer/asesoria/internal/db"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
)

func TestRetaBaseHistory(t *testing.T) {
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
		RetaBaseCents: 120000, RetaBaseFrom: "2026-01-01",
		RetaNetIncomeCents: 1800000, RetaNetIncomeYear: 2026,
	}
	if err := st.CreateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}
	if err := st.AddRetaBaseChange(ctx, taxpayer); err != nil {
		t.Fatal(err)
	}

	taxpayer.RetaBaseCents = 130000
	taxpayer.RetaBaseFrom = "2026-03-01"
	if err := st.UpdateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}
	if err := st.AddRetaBaseChange(ctx, taxpayer); err != nil {
		t.Fatal(err)
	}

	changes, err := st.ListRetaBaseChanges(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 history rows, got %d", len(changes))
	}
	if changes[0].BaseCents != 130000 || changes[0].BaseFrom != "2026-03-01" {
		t.Fatalf("unexpected first history row: %+v", changes[0])
	}
	if changes[1].BaseCents != 120000 || changes[1].BaseFrom != "2026-01-01" {
		t.Fatalf("unexpected second history row: %+v", changes[1])
	}
}
