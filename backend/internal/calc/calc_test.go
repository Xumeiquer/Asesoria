package calc

import (
	"testing"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

func TestCalc303GeneralRegime(t *testing.T) {
	invoices := []domain.Invoice{
		{Direction: domain.DirectionIssued, VATRate: 2100, BaseCents: 100_000, VATCents: 21_000},
		{Direction: domain.DirectionIssued, VATRate: 1000, BaseCents: 50_000, VATCents: 5_000},
		{Direction: domain.DirectionReceived, BaseCents: 40_000, VATCents: 8_400},
		{Direction: domain.DirectionReceived, Category: "inversion", BaseCents: 20_000, VATCents: 4_200},
	}

	computed, ok := Calculate("303", invoices, 0)
	if !ok {
		t.Fatal("expected 303 calculator")
	}
	values := computed["values"].(map[string]string)
	want := map[string]string{
		"01": "0.00",
		"03": "0.00",
		"04": "500.00",
		"06": "50.00",
		"07": "1000.00",
		"09": "210.00",
		"28": "400.00",
		"29": "84.00",
		"30": "200.00",
		"31": "42.00",
		"27": "260.00",
		"45": "126.00",
		"46": "134.00",
		"71": "134.00",
	}
	for casilla, expected := range want {
		if values[casilla] != expected {
			t.Errorf("casilla %s = %q, want %q", casilla, values[casilla], expected)
		}
	}
}

func TestCalc303RatesAndWarnings(t *testing.T) {
	invoices := []domain.Invoice{
		{Direction: domain.DirectionIssued, VATRate: 0, BaseCents: 10_000, VATCents: 0},
		{Direction: domain.DirectionIssued, VATRate: 400, BaseCents: 20_000, VATCents: 800},
		{Direction: domain.DirectionIssued, VATRate: 500, BaseCents: 30_000, VATCents: 1_500},
		{Direction: domain.DirectionIssued, VATRate: 1000, BaseCents: 40_000, VATCents: 4_000},
		{Direction: domain.DirectionIssued, VATRate: 2100, BaseCents: 50_000, VATCents: 10_500},
		{Direction: domain.DirectionIssued, VATRate: 700, BaseCents: 1_000, VATCents: 70},
	}

	computed, ok := Calculate("303", invoices, 0)
	if !ok {
		t.Fatal("expected 303 calculator")
	}
	values := computed["values"].(map[string]string)
	want := map[string]string{
		"150": "100.00",
		"152": "0.00",
		"01":  "200.00",
		"03":  "8.00",
		"04":  "400.00",
		"06":  "40.00",
		"07":  "500.00",
		"09":  "105.00",
		"27":  "153.00",
	}
	for casilla, expected := range want {
		if values[casilla] != expected {
			t.Errorf("casilla %s = %q, want %q", casilla, values[casilla], expected)
		}
	}
	warnings := computed["warnings"].([]string)
	if len(warnings) == 0 {
		t.Fatal("expected a warning for the unsupported VAT rate")
	}
}

func TestCalc130AppliesPreviousPayments(t *testing.T) {
	invoices := []domain.Invoice{
		{Direction: domain.DirectionIssued, BaseCents: 100_000, VATCents: 21_000, RetentionCents: 5_000},
		{Direction: domain.DirectionReceived, BaseCents: 20_000, VATCents: 4_200},
	}

	computed, ok := Calculate("130", invoices, 3_000)
	if !ok {
		t.Fatal("expected 130 calculator")
	}
	values := computed["values"].(map[string]string)
	want := map[string]string{
		"01": "1000.00",
		"02": "200.00",
		"03": "800.00",
		"04": "160.00",
		"05": "50.00",
		"06": "30.00",
		"07": "80.00",
	}
	for casilla, expected := range want {
		if values[casilla] != expected {
			t.Errorf("casilla %s = %q, want %q", casilla, values[casilla], expected)
		}
	}
}

func TestCalcUnknownModel(t *testing.T) {
	if _, ok := Calculate("111", nil, 0); ok {
		t.Fatal("expected no calculator for 111")
	}
}
