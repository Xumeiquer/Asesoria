package store_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Xumeiquer/asesoria/internal/db"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
)

func testStore(t *testing.T) (*store.Store, context.Context) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return store.New(conn), context.Background()
}

func testTaxpayer(t *testing.T, st *store.Store, ctx context.Context) domain.Taxpayer {
	t.Helper()
	taxpayer := domain.Taxpayer{
		NIF: "12345678Z", Name: "Test", LegalForm: domain.LegalFormAutonomo,
		VATRegime: "general", IRPFMethod: "directa", EconomicActivity: true,
		FiscalYearEnd: "12-31", ActivityStart: "2026-01-01",
	}
	if err := st.CreateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}
	return taxpayer
}

func TestBulkSetFilingStatusTracksPresentedWithoutReceipt(t *testing.T) {
	st, ctx := testStore(t)
	taxpayer := testTaxpayer(t, st, ctx)

	filings := []domain.Filing{
		{TaxpayerID: taxpayer.ID, ModelCode: "303", ModelName: "IVA", Entity: "AEAT",
			PeriodLabel: "1T 2026", PeriodStart: "2026-01-01", PeriodEnd: "2026-03-31",
			DueDate: "2026-04-20", Status: domain.StatusPending},
		{TaxpayerID: taxpayer.ID, ModelCode: "303", ModelName: "IVA", Entity: "AEAT",
			PeriodLabel: "2T 2026", PeriodStart: "2026-04-01", PeriodEnd: "2026-06-30",
			DueDate: "2026-07-20", Status: domain.StatusPending},
	}
	if _, err := st.UpsertFilings(ctx, filings); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListFilings(ctx, store.FilingFilter{TaxpayerID: taxpayer.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 filings, got %d", len(list))
	}
	ids := []string{list[0].ID, list[1].ID}
	updated, err := st.BulkSetFilingStatus(ctx, ids, domain.StatusPresented)
	if err != nil {
		t.Fatal(err)
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated filings, got %d", updated)
	}
	presented, err := st.GetFiling(ctx, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if presented.Status != domain.StatusPresented || presented.PresentedAt == nil {
		t.Fatalf("filing was not marked as presented: %+v", presented)
	}
	if presented.ReceiptDocumentID != nil {
		t.Fatal("bulk presentation must not invent a receipt")
	}

	if _, err := st.BulkSetFilingStatus(ctx, ids, domain.StatusPending); err != nil {
		t.Fatal(err)
	}
	reopened, err := st.GetFiling(ctx, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Status != domain.StatusPending || reopened.PresentedAt != nil || reopened.ReceiptDocumentID != nil {
		t.Fatalf("reopening did not clear presentation metadata: %+v", reopened)
	}
}

func TestUpdateInvoiceRecalculatesTotalsAndClearsDocument(t *testing.T) {
	st, ctx := testStore(t)
	taxpayer := testTaxpayer(t, st, ctx)

	document := &domain.Document{
		TaxpayerID: &taxpayer.ID, Kind: "factura_recibida", Filename: "factura.pdf",
		MIME: "application/pdf", Size: 10, SHA256: "abc",
	}
	if err := st.CreateDocument(ctx, document); err != nil {
		t.Fatal(err)
	}
	docID := document.ID
	invoice := domain.Invoice{
		TaxpayerID: taxpayer.ID, Direction: domain.DirectionReceived, Number: "F-1",
		InvoiceDate: "2026-10-01", CounterpartyName: "Proveedor", BaseCents: 10000,
		VATRate: 2100, VATCents: 2100, TotalCents: 12100, DocumentID: &docID,
	}
	if err := st.CreateInvoice(ctx, &invoice); err != nil {
		t.Fatal(err)
	}

	invoice.Number = "F-1-bis"
	invoice.BaseCents = 20000
	invoice.VATRate = 1000
	invoice.VATCents = 2000
	invoice.TotalCents = 22000
	invoice.DocumentID = nil
	if err := st.UpdateInvoice(ctx, &invoice); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetInvoice(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != "F-1-bis" || got.BaseCents != 20000 || got.VATCents != 2000 || got.TotalCents != 22000 {
		t.Fatalf("invoice was not updated: %+v", got)
	}
	if got.DocumentID != nil {
		t.Fatalf("document link was not cleared: %v", *got.DocumentID)
	}
}

func TestTaxpayerCNAEsRoundTrip(t *testing.T) {
	st, ctx := testStore(t)
	taxpayer := domain.Taxpayer{
		NIF: "12345678Z", Name: "Test CNAE", LegalForm: domain.LegalFormAutonomo,
		VATRegime: "general", IRPFMethod: "directa", EconomicActivity: true,
		CNAEs: []string{"7022", "6201"}, FiscalYearEnd: "12-31", ActivityStart: "2026-01-01",
	}
	if err := st.CreateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetTaxpayer(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.CNAEs, []string{"7022", "6201"}) {
		t.Fatalf("CNAEs were not persisted: %+v", got.CNAEs)
	}

	taxpayer.CNAEs = []string{"6920"}
	if err := st.UpdateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetTaxpayer(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.CNAEs, []string{"6920"}) {
		t.Fatalf("CNAEs were not updated: %+v", got.CNAEs)
	}
}
