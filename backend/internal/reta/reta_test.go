package reta

import "testing"

func TestBracketFor2026Boundaries(t *testing.T) {
	tests := []struct {
		name       string
		monthly    int64
		wantTable  string
		wantNumber int
	}{
		{"sin ingresos", 0, TableReduced, 1},
		{"limite superior reducido 1", 67000, TableReduced, 1},
		{"inicio reducido 2", 67001, TableReduced, 2},
		{"limite reducido 2", 90000, TableReduced, 2},
		{"inicio reducido 3", 90001, TableReduced, 3},
		{"ultimo centimo reducido 3", 116669, TableReduced, 3},
		{"inicio general 1", 116670, TableGeneral, 1},
		{"limite general 1", 130000, TableGeneral, 1},
		{"inicio general 2", 130001, TableGeneral, 2},
		{"limite general 11", 600000, TableGeneral, 11},
		{"inicio general 12", 600001, TableGeneral, 12},
		{"sin limite superior", 900000, TableGeneral, 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bracket, ok := BracketFor(2026, tt.monthly*12)
			if !ok {
				t.Fatal("BracketFor returned no bracket")
			}
			if bracket.Table != tt.wantTable || bracket.Number != tt.wantNumber {
				t.Fatalf("got tramo %d (%s), want tramo %d (%s)", bracket.Number, bracket.Table, tt.wantNumber, tt.wantTable)
			}
		})
	}
}

func TestValidateBase(t *testing.T) {
	tests := []struct {
		name      string
		annual    int64
		base      int64
		wantError bool
		wantMin   int64
		wantMax   int64
	}{
		{"base valida en tramo reducido", 700000, 70000, false, 65359, 71894},
		{"base por debajo del tramo reducido", 700000, 60000, true, 65359, 71894},
		{"base valida en tramo general", 1800000, 100000, false, 96078, 150000},
		{"base por encima del tramo general", 1800000, 160000, true, 96078, 150000},
		{"limite inferior exacto", 1400040, 95098, false, 95098, 130000},
		{"un centimo por debajo", 1400040, 95097, true, 95098, 130000},
		{"limite superior exacto", 1400040, 130000, false, 95098, 130000},
		{"un centimo por encima", 1400040, 130001, true, 95098, 130000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bracket, err := ValidateBase(2026, tt.annual, tt.base)
			if tt.wantError && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bracket.MinBaseCents != tt.wantMin || bracket.MaxBaseCents != tt.wantMax {
				t.Fatalf("got base range %d-%d, want %d-%d", bracket.MinBaseCents, bracket.MaxBaseCents, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestValidateBaseUnsupportedYear(t *testing.T) {
	if _, err := ValidateBase(2025, 1800000, 100000); err == nil {
		t.Fatal("expected an error for an unsupported year")
	}
}

func TestFormatEUR(t *testing.T) {
	if got := FormatEUR(116670); got != "1.166,70 €" {
		t.Fatalf("FormatEUR(116670) = %q", got)
	}
	if got := FormatEUR(65359); got != "653,59 €" {
		t.Fatalf("FormatEUR(65359) = %q", got)
	}
	if got := FormatEUR(-123456); got != "-1.234,56 €" {
		t.Fatalf("FormatEUR(-123456) = %q", got)
	}
}
