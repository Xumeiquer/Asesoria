package api

import (
	"slices"
	"testing"
)

func validRetaInput() taxpayerInput {
	return taxpayerInput{
		NIF:                "12345678Z",
		Name:               "Lucía Prueba",
		LegalForm:          "autonomo",
		RetaBaseCents:      100000,
		RetaBaseFrom:       "2026-01-01",
		RetaNetIncomeCents: 1800000,
		RetaNetIncomeYear:  2026,
	}
}

func TestTaxpayerInputRetaProfile(t *testing.T) {
	t.Run("perfil válido", func(t *testing.T) {
		got, msg := validRetaInput().toDomain()
		if msg != "" {
			t.Fatalf("unexpected validation error: %s", msg)
		}
		if got.RetaBaseCents != 100000 || got.RetaNetIncomeYear != 2026 {
			t.Fatalf("RETA profile was not copied: %+v", got)
		}
	})

	t.Run("base fuera del tramo", func(t *testing.T) {
		in := validRetaInput()
		in.RetaBaseCents = 80000
		if _, msg := in.toDomain(); msg == "" {
			t.Fatal("expected a validation error")
		}
	})

	t.Run("base sin fecha de vigencia", func(t *testing.T) {
		in := validRetaInput()
		in.RetaBaseFrom = ""
		if _, msg := in.toDomain(); msg == "" {
			t.Fatal("expected a validation error")
		}
	})

	t.Run("perfil vacío permitido", func(t *testing.T) {
		in := validRetaInput()
		in.RetaBaseCents = 0
		in.RetaBaseFrom = ""
		in.RetaNetIncomeCents = 0
		in.RetaNetIncomeYear = 0
		if _, msg := in.toDomain(); msg != "" {
			t.Fatalf("unexpected validation error: %s", msg)
		}
	})

	t.Run("solo fecha y año no activan el perfil", func(t *testing.T) {
		in := validRetaInput()
		in.RetaBaseCents = 0
		in.RetaNetIncomeCents = 0
		got, msg := in.toDomain()
		if msg != "" {
			t.Fatalf("unexpected validation error: %s", msg)
		}
		if got.RetaBaseFrom != "" || got.RetaNetIncomeYear != 0 {
			t.Fatalf("stray RETA fields were not cleared: %+v", got)
		}
	})

	t.Run("sociedad limpia el perfil RETA", func(t *testing.T) {
		in := validRetaInput()
		in.LegalForm = "sl"
		got, msg := in.toDomain()
		if msg != "" {
			t.Fatalf("unexpected validation error: %s", msg)
		}
		if got.RetaBaseCents != 0 || got.RetaBaseFrom != "" || got.RetaNetIncomeCents != 0 || got.RetaNetIncomeYear != 0 {
			t.Fatalf("SL kept RETA data: %+v", got)
		}
	})
}

func TestTaxpayerInputMultipleCNAEs(t *testing.T) {
	in := validRetaInput()
	in.CNAEs = []string{"7022", " 6201 ", "7022", ""}
	got, msg := in.toDomain()
	if msg != "" {
		t.Fatalf("unexpected validation error: %s", msg)
	}
	if !slices.Equal(got.CNAEs, []string{"7022", "6201"}) {
		t.Fatalf("unexpected CNAEs: %+v", got.CNAEs)
	}

	legacy := validRetaInput()
	legacy.CNAE = "6920"
	got, msg = legacy.toDomain()
	if msg != "" {
		t.Fatalf("unexpected legacy validation error: %s", msg)
	}
	if !slices.Equal(got.CNAEs, []string{"6920"}) {
		t.Fatalf("legacy CNAE was not converted: %+v", got.CNAEs)
	}

	invalid := validRetaInput()
	invalid.CNAEs = []string{"702"}
	if _, msg := invalid.toDomain(); msg == "" {
		t.Fatal("expected invalid CNAE to be rejected")
	}
}
