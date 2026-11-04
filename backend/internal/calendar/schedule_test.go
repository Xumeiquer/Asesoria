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

	monthlyVAT := g.periodsFor(m303, domain.Taxpayer{VATRegime: "general", MonthlyVAT: true}, 2025)
	if len(monthlyVAT) != 12 {
		t.Fatalf("303 monthly-VAT periods = %d, want 12", len(monthlyVAT))
	}
}

func TestMonthlyWithholdings111And115(t *testing.T) {
	c := loadCatalog(t)
	g := New(c)
	tp := domain.Taxpayer{
		VATRegime: "general", EconomicActivity: true,
		WithholdsProfessionalIncome: true, HasRentals: true,
		MonthlyWithholdings: true,
	}
	for _, code := range []string{"111", "115"} {
		m := modelByCode(t, c, code)
		periods := g.periodsFor(m, tp, 2025)
		if len(periods) != 12 {
			t.Errorf("%s monthly periods = %d, want 12", code, len(periods))
		}
		p := periodByLabel(t, periods, "Diciembre 2024")
		if got := p.due.Format("2006-01-02"); got != "2025-01-20" {
			t.Errorf("%s December 2024 due = %s, want 2025-01-20", code, got)
		}
	}

	tp.MonthlyWithholdings = false
	for _, code := range []string{"111", "115"} {
		m := modelByCode(t, c, code)
		if periods := g.periodsFor(m, tp, 2025); len(periods) != 4 {
			t.Errorf("%s quarterly periods = %d, want 4", code, len(periods))
		}
	}
}

func TestIntracommunityMonthly349(t *testing.T) {
	c := loadCatalog(t)
	g := New(c)
	m349 := modelByCode(t, c, "349")
	tp := domain.Taxpayer{
		VATRegime: "general", EconomicActivity: true,
		HasIntracommunityOperations: true, IntracommunityOver50K: true,
	}

	periods := g.periodsFor(m349, tp, 2027)
	if len(periods) != 12 {
		t.Fatalf("349 monthly periods = %d, want 12", len(periods))
	}
	pJul := periodByLabel(t, periods, "Julio 2027")
	if got := pJul.due.Format("2006-01-02"); got != "2027-09-20" {
		t.Errorf("349 July 2027 due = %s, want 2027-09-20", got)
	}
	pAug := periodByLabel(t, periods, "Agosto 2027")
	if got := pAug.due.Format("2006-01-02"); got != "2027-09-20" {
		t.Errorf("349 August 2027 due = %s, want 2027-09-20", got)
	}
	pDec := periodByLabel(t, periods, "Diciembre 2026")
	if got := pDec.due.Format("2006-01-02"); got != "2027-02-01" {
		t.Errorf("349 December 2026 due = %s, want 2027-02-01 (30 January moved from Saturday)", got)
	}

	tp.IntracommunityOver50K = false
	if quarterly := g.periodsFor(m349, tp, 2027); len(quarterly) != 4 {
		t.Errorf("349 quarterly periods = %d, want 4", len(quarterly))
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
	base.MonthlyVAT = true
	monthlyVAT := g.Generate(base, 2026)
	if got := countModel(monthlyVAT, "303"); got != 12 {
		t.Errorf("monthly VAT 303 filings = %d, want 12", got)
	}
	if got := countModel(monthlyVAT, "390"); got != 1 {
		t.Errorf("monthly VAT 390 filings = %d, want 1", got)
	}

	base.MonthlyVAT = false
	base.HasIntracommunityOperations = true
	intra := g.Generate(base, 2026)
	if got := countModel(intra, "349"); got != 4 {
		t.Errorf("intracommunity 349 filings = %d, want 4", got)
	}
	base.IntracommunityOver50K = true
	intraMonthly := g.Generate(base, 2026)
	if got := countModel(intraMonthly, "349"); got != 12 {
		t.Errorf("monthly intracommunity 349 filings = %d, want 12", got)
	}

	base.HasIntracommunityOperations = false
	base.IntracommunityOver50K = false
	base.WithholdsProfessionalIncome = true
	withholding := g.Generate(base, 2026)
	if got := countModel(withholding, "111"); got != 4 {
		t.Errorf("professional withholding 111 filings = %d, want 4", got)
	}
	if got := countModel(withholding, "190"); got != 1 {
		t.Errorf("professional withholding 190 filings = %d, want 1", got)
	}
	base.HasRentals = true
	base.MonthlyWithholdings = true
	monthlyWithholdings := g.Generate(base, 2026)
	if got := countModel(monthlyWithholdings, "111"); got != 12 {
		t.Errorf("monthly withholding 111 filings = %d, want 12", got)
	}
	if got := countModel(monthlyWithholdings, "115"); got != 12 {
		t.Errorf("monthly withholding 115 filings = %d, want 12", got)
	}
	if got := countModel(monthlyWithholdings, "190"); got != 1 {
		t.Errorf("monthly withholding 190 filings = %d, want 1", got)
	}

	base.WithholdsProfessionalIncome = false
	base.HasRentals = false
	base.MonthlyWithholdings = false
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
	recargo.HasGeneralVATActivity = true
	combined := g.Generate(recargo, 2026)
	if got := countModel(combined, "303"); got != 4 {
		t.Errorf("recargo with general activity 303 filings = %d, want 4", got)
	}
	if got := countModel(combined, "390"); got != 1 {
		t.Errorf("recargo with general activity 390 filings = %d, want 1", got)
	}
}
