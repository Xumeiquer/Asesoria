package calendar

import (
	"testing"

	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/domain"
)

func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return c
}

func modelByCode(t *testing.T, c *catalog.Catalog, code string) catalog.Model {
	t.Helper()
	m, ok := c.Get(code)
	if !ok {
		t.Fatalf("model %s not found", code)
	}
	return m
}

func periodByLabel(t *testing.T, periods []period, label string) period {
	t.Helper()
	for _, p := range periods {
		if p.label == label {
			return p
		}
	}
	t.Fatalf("period %q not found", label)
	return period{}
}

func TestQuarterlyLastPeriodUsesDueDays(t *testing.T) {
	c := loadCatalog(t)
	g := New(c)
	tp := domain.Taxpayer{VATRegime: "general", EconomicActivity: true}
	for _, code := range []string{"303", "130", "131", "349", "309"} {
		m := modelByCode(t, c, code)
		p := periodByLabel(t, g.periodsFor(m, tp, 2025), "4T 2024")
		if got := p.due.Format("2006-01-02"); got != "2025-01-30" {
			t.Errorf("%s 4T 2024 due = %s, want 2025-01-30", code, got)
		}
	}
}

func TestAnnualDueDates(t *testing.T) {
	c := loadCatalog(t)
	g := New(c)
	tp := domain.Taxpayer{VATRegime: "general", EconomicActivity: true}

	m390 := modelByCode(t, c, "390")
	p390 := periodByLabel(t, g.periodsFor(m390, tp, 2025), "2024")
	if got := p390.due.Format("2006-01-02"); got != "2025-01-30" {
		t.Errorf("390 due = %s, want 2025-01-30", got)
	}

	m347 := modelByCode(t, c, "347")
	p347 := periodByLabel(t, g.periodsFor(m347, tp, 2025), "2024")
	if got := p347.due.Format("2006-01-02"); got != "2025-02-28" {
		t.Errorf("347 due = %s, want 2025-02-28", got)
	}
	p347Leap := periodByLabel(t, g.periodsFor(m347, tp, 2024), "2023")
	if got := p347Leap.due.Format("2006-01-02"); got != "2024-02-29" {
		t.Errorf("347 leap-year due = %s, want 2024-02-29", got)
	}
}

func TestSIIMonthly303(t *testing.T) {
	c := loadCatalog(t)
	g := New(c)
	m303 := modelByCode(t, c, "303")

	quarterly := g.periodsFor(m303, domain.Taxpayer{VATRegime: "general"}, 2025)
	if len(quarterly) != 4 {
		t.Fatalf("303 quarterly periods = %d, want 4", len(quarterly))
	}

	monthly := g.periodsFor(m303, domain.Taxpayer{VATRegime: "general", UsesSII: true}, 2025)
	if len(monthly) != 12 {
		t.Fatalf("303 SII periods = %d, want 12", len(monthly))
	}
	pDec := periodByLabel(t, monthly, "Diciembre 2024")
	if got := pDec.due.Format("2006-01-02"); got != "2025-01-30" {
		t.Errorf("303 December 2024 due = %s, want 2025-01-30", got)
	}
	pJan := periodByLabel(t, monthly, "Enero 2025")
	if got := pJan.due.Format("2006-01-02"); got != "2025-02-28" {
		t.Errorf("303 January 2025 due = %s, want 2025-02-28", got)
	}
}

func countModel(filings []domain.Filing, code string) int {
	n := 0
	for _, f := range filings {
		if f.ModelCode == code {
			n++
		}
	}
	return n
}

func TestGenerateAppliesTaxProfile(t *testing.T) {
	c := loadCatalog(t)
	g := New(c)
	base := domain.Taxpayer{
		LegalForm: domain.LegalFormAutonomo, VATRegime: "general",
		IRPFMethod: "directa", EconomicActivity: true, ActivityStart: "2025-01-01",
	}

	general := g.Generate(base, 2026)
	if got := countModel(general, "303"); got != 4 {
		t.Errorf("general 303 filings = %d, want 4", got)
	}
	if got := countModel(general, "390"); got != 1 {
		t.Errorf("general 390 filings = %d, want 1", got)
	}
	if got := countModel(general, "349"); got != 0 {
		t.Errorf("general 349 filings = %d, want 0", got)
	}
	if got := countModel(general, "111"); got != 0 {
		t.Errorf("general 111 filings = %d, want 0", got)
	}

	base.UsesSII = true
	sii := g.Generate(base, 2026)
	if got := countModel(sii, "303"); got != 12 {
		t.Errorf("SII 303 filings = %d, want 12", got)
	}
	if got := countModel(sii, "390"); got != 0 {
		t.Errorf("SII 390 filings = %d, want 0", got)
	}

	base.UsesSII = false
	base.HasIntracommunityOperations = true
	intra := g.Generate(base, 2026)
	if got := countModel(intra, "349"); got != 4 {
		t.Errorf("intracommunity 349 filings = %d, want 4", got)
	}

	base.HasIntracommunityOperations = false
	base.WithholdsProfessionalIncome = true
	withholding := g.Generate(base, 2026)
	if got := countModel(withholding, "111"); got != 4 {
		t.Errorf("professional withholding 111 filings = %d, want 4", got)
	}
	if got := countModel(withholding, "190"); got != 1 {
		t.Errorf("professional withholding 190 filings = %d, want 1", got)
	}

	base.WithholdsProfessionalIncome = false
	base.ExemptFromIRPFFractionalPayment = true
	exempt := g.Generate(base, 2026)
	if got := countModel(exempt, "130"); got != 0 {
		t.Errorf("fractional-payment exemption 130 filings = %d, want 0", got)
	}

	recargo := base
	recargo.ExemptFromIRPFFractionalPayment = false
	recargo.VATRegime = "recargo"
	recargoFilings := g.Generate(recargo, 2026)
	if got := countModel(recargoFilings, "303"); got != 0 {
		t.Errorf("recargo 303 filings = %d, want 0", got)
	}
	if got := countModel(recargoFilings, "390"); got != 0 {
		t.Errorf("recargo 390 filings = %d, want 0", got)
	}
}
