// Package reta implements the general 2026 contribution brackets for the
// Spanish Régimen Especial de Trabajadores Autónomos (RETA).
//
// The brackets are kept as integers in cents to avoid floating-point
// comparisons. MaxIncomeCents == 0 means the last bracket has no upper limit.
// The table covers the general and reduced tables published by the Seguridad
// Social; it does not model special cases such as pluriactivity, artistic
// activities or the special rules for family collaborators.
package reta

import "fmt"

const (
	TableReduced = "reducida"
	TableGeneral = "general"
)

// Bracket is one net-income bracket and the contribution base range it allows.
type Bracket struct {
	Number         int    `json:"number"`
	Table          string `json:"table"`
	MinIncomeCents int64  `json:"min_income_cents"`
	MaxIncomeCents int64  `json:"max_income_cents"` // 0 means no upper limit
	MinBaseCents   int64  `json:"min_base_cents"`
	MaxBaseCents   int64  `json:"max_base_cents"`
}

// Label returns a human-readable name such as "tramo 2 (tabla general)".
func (b Bracket) Label() string {
	return fmt.Sprintf("tramo %d (tabla %s)", b.Number, b.Table)
}

var brackets2026 = []Bracket{
	{Number: 1, Table: TableReduced, MinIncomeCents: 0, MaxIncomeCents: 67000, MinBaseCents: 65359, MaxBaseCents: 71894},
	{Number: 2, Table: TableReduced, MinIncomeCents: 67001, MaxIncomeCents: 90000, MinBaseCents: 71895, MaxBaseCents: 90000},
	{Number: 3, Table: TableReduced, MinIncomeCents: 90001, MaxIncomeCents: 116669, MinBaseCents: 84967, MaxBaseCents: 116670},
	{Number: 1, Table: TableGeneral, MinIncomeCents: 116670, MaxIncomeCents: 130000, MinBaseCents: 95098, MaxBaseCents: 130000},
	{Number: 2, Table: TableGeneral, MinIncomeCents: 130001, MaxIncomeCents: 150000, MinBaseCents: 96078, MaxBaseCents: 150000},
	{Number: 3, Table: TableGeneral, MinIncomeCents: 150001, MaxIncomeCents: 170000, MinBaseCents: 96078, MaxBaseCents: 170000},
	{Number: 4, Table: TableGeneral, MinIncomeCents: 170001, MaxIncomeCents: 185000, MinBaseCents: 114379, MaxBaseCents: 185000},
	{Number: 5, Table: TableGeneral, MinIncomeCents: 185001, MaxIncomeCents: 203000, MinBaseCents: 120915, MaxBaseCents: 203000},
	{Number: 6, Table: TableGeneral, MinIncomeCents: 203001, MaxIncomeCents: 233000, MinBaseCents: 127451, MaxBaseCents: 233000},
	{Number: 7, Table: TableGeneral, MinIncomeCents: 233001, MaxIncomeCents: 276000, MinBaseCents: 135621, MaxBaseCents: 276000},
	{Number: 8, Table: TableGeneral, MinIncomeCents: 276001, MaxIncomeCents: 319000, MinBaseCents: 143791, MaxBaseCents: 319000},
	{Number: 9, Table: TableGeneral, MinIncomeCents: 319001, MaxIncomeCents: 362000, MinBaseCents: 151961, MaxBaseCents: 362000},
	{Number: 10, Table: TableGeneral, MinIncomeCents: 362001, MaxIncomeCents: 405000, MinBaseCents: 160131, MaxBaseCents: 405000},
	{Number: 11, Table: TableGeneral, MinIncomeCents: 405001, MaxIncomeCents: 600000, MinBaseCents: 173203, MaxBaseCents: 510120},
	{Number: 12, Table: TableGeneral, MinIncomeCents: 600001, MaxIncomeCents: 0, MinBaseCents: 192810, MaxBaseCents: 510120},
}

// Brackets returns a copy of the general and reduced brackets for the year.
func Brackets(year int) ([]Bracket, bool) {
	if year != 2026 {
		return nil, false
	}
	out := make([]Bracket, len(brackets2026))
	copy(out, brackets2026)
	return out, true
}

// MonthlyIncomeCents converts annual net income to the monthly average used by
// the brackets. Negative income is treated as zero. The result is rounded to
// the nearest cent.
func MonthlyIncomeCents(annualCents int64) int64 {
	if annualCents <= 0 {
		return 0
	}
	return (annualCents + 6) / 12
}

// BracketFor returns the bracket corresponding to the annual net-income
// forecast. The second result is false when there is no table for the year.
func BracketFor(year int, annualNetIncomeCents int64) (Bracket, bool) {
	brackets, ok := Brackets(year)
	if !ok {
		return Bracket{}, false
	}
	monthly := MonthlyIncomeCents(annualNetIncomeCents)
	for _, bracket := range brackets {
		if monthly < bracket.MinIncomeCents {
			continue
		}
		if bracket.MaxIncomeCents == 0 || monthly <= bracket.MaxIncomeCents {
			return bracket, true
		}
	}
	return Bracket{}, false
}

// ValidateBase checks that baseCents is within the range allowed by the
// bracket derived from annualNetIncomeCents.
func ValidateBase(year int, annualNetIncomeCents, baseCents int64) (Bracket, error) {
	bracket, ok := BracketFor(year, annualNetIncomeCents)
	if !ok {
		return Bracket{}, fmt.Errorf("no hay tabla de tramos de RETA para %d", year)
	}
	if baseCents < bracket.MinBaseCents || baseCents > bracket.MaxBaseCents {
		return bracket, fmt.Errorf(
			"la base de %s no está dentro del %s (mínima %s, máxima %s)",
			FormatEUR(baseCents), bracket.Label(), FormatEUR(bracket.MinBaseCents), FormatEUR(bracket.MaxBaseCents),
		)
	}
	return bracket, nil
}

// FormatEUR formats cents using Spanish conventions, for example 1166.7 ->
// "1.166,70 €".
func FormatEUR(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	euros := cents / 100
	decimals := cents % 100
	return fmt.Sprintf("%s%s,%02d €", sign, formatThousands(euros), decimals)
}

func formatThousands(value int64) string {
	raw := fmt.Sprintf("%d", value)
	if len(raw) <= 3 {
		return raw
	}
	out := make([]byte, 0, len(raw)+len(raw)/3)
	first := len(raw) % 3
	if first == 0 {
		first = 3
	}
	out = append(out, raw[:first]...)
	for i := first; i < len(raw); i += 3 {
		out = append(out, '.')
		out = append(out, raw[i:i+3]...)
	}
	return string(out)
}
