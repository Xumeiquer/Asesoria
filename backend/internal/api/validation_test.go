package api

import (
	"testing"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

func TestTaxpayerInputValidatesIdentityAndActivity(t *testing.T) {
	valid := validRetaInput()
	valid.ActivityStart = "2026-01-15"
	valid.IBAN = "ES9121000418450200051332"
	if _, msg := valid.toDomain(); msg != "" {
		t.Fatalf("unexpected validation error: %s", msg)
	}

	tests := []struct {
		name   string
		mutate func(*taxpayerInput)
	}{
		{"nif inválido", func(in *taxpayerInput) { in.NIF = "1234" }},
		{"iban inválido", func(in *taxpayerInput) { in.IBAN = "ES001234" }},
		{"inicio de actividad inválido", func(in *taxpayerInput) { in.ActivityStart = "15/01/2026" }},
		{"cierre fiscal inválido", func(in *taxpayerInput) { in.FiscalYearEnd = "13-01" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			in.FiscalYearEnd = "12-31"
			tc.mutate(&in)
			if _, msg := in.toDomain(); msg == "" {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestEditableStatusExcludesPresentationAndLegacyLate(t *testing.T) {
	for _, status := range []string{domain.StatusPending, domain.StatusInProgress, domain.StatusReady, domain.StatusNotApplicable} {
		if !editableStatus(status) {
			t.Errorf("%s should be editable", status)
		}
	}
	for _, status := range []string{domain.StatusPresented, domain.StatusLate, "unknown"} {
		if editableStatus(status) {
			t.Errorf("%s should not be editable directly", status)
		}
	}
}

func TestInvoiceFromInputValidatesAndCalculates(t *testing.T) {
	docID := " doc-1 "
	inv, msg := invoiceFromInput(invoiceInput{
		TaxpayerID: " taxpayer-1 ", Direction: domain.DirectionIssued,
		Number: " F-1 ", InvoiceDate: "2026-10-01",
		CounterpartyNIF: " b12345678 ", CounterpartyName: " Cliente ",
		BaseCents: 10000, VATRate: 2100, RetentionRate: 1500, DocumentID: &docID,
	})
	if msg != "" {
		t.Fatalf("unexpected validation error: %s", msg)
	}
	if inv.TotalCents != 10600 {
		t.Fatalf("unexpected total: %d", inv.TotalCents)
	}
	if inv.DocumentID == nil || *inv.DocumentID != "doc-1" {
		t.Fatalf("document id was not normalized: %+v", inv.DocumentID)
	}
	if inv.CounterpartyNIF != "B12345678" || inv.Number != "F-1" {
		t.Fatalf("text fields were not normalized: %+v", inv)
	}

	bad := invoiceInput{
		TaxpayerID: "taxpayer-1", Direction: domain.DirectionIssued,
		InvoiceDate: "2026-10-01", BaseCents: 1000, VATRate: 2100,
		CounterpartyNIF: "no-es-un-nif",
	}
	if _, msg := invoiceFromInput(bad); msg == "" {
		t.Fatal("expected invalid counterparty NIF to be rejected")
	}
}
