package calendar_test

import (
	"testing"

	"github.com/Xumeiquer/asesoria/internal/calendar"
	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/domain"
)

func TestGenerateSkipsPeriodsBeforeActivityStart(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	g := calendar.New(cat)
	taxpayer := domain.Taxpayer{
		NIF: "12345678Z", Name: "Actividad tardía", LegalForm: domain.LegalFormAutonomo,
		VATRegime: "general", IRPFMethod: "directa", EconomicActivity: true,
		ActivityStart: "2026-04-01",
	}

	filings := g.Generate(taxpayer, 2026)
	if len(filings) == 0 {
		t.Fatal("expected filings for 2026")
	}
	for _, filing := range filings {
		if filing.PeriodEnd < taxpayer.ActivityStart {
			t.Errorf("generated %s %s ending %s before activity start %s",
				filing.ModelCode, filing.PeriodLabel, filing.PeriodEnd, taxpayer.ActivityStart)
		}
		if filing.ModelCode == "303" && filing.PeriodLabel == "1T 2026" {
			t.Error("1T 2026 should not be generated when activity starts in April")
		}
	}

	taxpayer.ActivityStart = ""
	all := g.Generate(taxpayer, 2026)
	if len(all) <= len(filings) {
		t.Fatalf("expected more filings without activity start: before=%d after=%d", len(filings), len(all))
	}
}
