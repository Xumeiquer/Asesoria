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
		"01": "1000.00",
		"04": "500.00",
		"16": "260.00",
		"18": "42.00",
		"24": "84.00",
		"27": "126.00",
		"28": "134.00",
	}
	for casilla, expected := range want {
		if values[casilla] != expected {
			t.Errorf("casilla %s = %q, want %q", casilla, values[casilla], expected)
		}
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
