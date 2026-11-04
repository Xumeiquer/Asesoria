package catalog

import (
	"strings"
	"testing"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

func TestLoadInheritsCatalogUpdatedAt(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.UpdatedAt == "" {
		t.Fatal("catalog updated_at is empty")
	}
	for _, m := range c.All() {
		if m.UpdatedAt == "" {
			t.Errorf("model %s has no updated_at", m.Code)
		}
	}
}

func TestModelsHaveDescriptionAndOfficialURL(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range c.All() {
		if strings.TrimSpace(m.Description) == "" {
			t.Errorf("model %s has no description", m.Code)
		}
		if !strings.HasPrefix(m.URL, "https://") {
			t.Errorf("model %s has a non-HTTPS URL: %q", m.Code, m.URL)
		}
		if strings.TrimSpace(m.Periodicity) == "" {
			t.Errorf("model %s has no periodicity", m.Code)
		}
	}
}

func TestEveryCatalogueDocumentHasAKindAndFolder(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range c.All() {
		if len(m.DocumentRequirements) != len(m.Documents) {
			t.Errorf("model %s has %d documents and %d requirements", m.Code, len(m.Documents), len(m.DocumentRequirements))
		}
		for _, req := range m.DocumentRequirements {
			if req.Kind == "" || req.Kind == "other" {
				t.Errorf("model %s document %q has no specific kind", m.Code, req.Label)
			}
			if req.Folder == "" || req.Folder == "Otros" {
				t.Errorf("model %s document %q has no specific folder", m.Code, req.Label)
			}
		}
	}
}

func modelApplies(t *testing.T, c *Catalog, code string, taxpayer domain.Taxpayer) bool {
	t.Helper()
	m, ok := c.Get(code)
	if !ok {
		t.Fatalf("model %s not found", code)
	}
	return m.AppliesTo(taxpayer)
}

func baseAutonomo() domain.Taxpayer {
	return domain.Taxpayer{
		NIF: "12345678Z", Name: "Prueba", LegalForm: domain.LegalFormAutonomo,
		VATRegime: "general", IRPFMethod: "directa", EconomicActivity: true,
	}
}

func TestApplicabilityForAutonomoProfiles(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	t.Run("303 general, simplificado o actividad general adicional", func(t *testing.T) {
		for _, regime := range []string{"general", "simplificado"} {
			tp := baseAutonomo()
			tp.VATRegime = regime
			if !modelApplies(t, c, "303", tp) {
				t.Errorf("303 should apply to %s", regime)
			}
		}
		for _, regime := range []string{"recargo", "exento"} {
			tp := baseAutonomo()
			tp.VATRegime = regime
			if modelApplies(t, c, "303", tp) {
				t.Errorf("303 should not apply to %s", regime)
			}
			tp.HasGeneralVATActivity = true
			if !modelApplies(t, c, "303", tp) {
				t.Errorf("303 should apply to %s with an additional general-regime activity", regime)
			}
		}
	})

	t.Run("390 solo general y sin SII", func(t *testing.T) {
		tp := baseAutonomo()
		if !modelApplies(t, c, "390", tp) {
			t.Error("390 should apply to a general regime taxpayer")
		}
		tp.UsesSII = true
		if modelApplies(t, c, "390", tp) {
			t.Error("390 should not apply when the taxpayer uses SII")
		}
		for _, regime := range []string{"simplificado", "recargo", "exento"} {
			tp = baseAutonomo()
			tp.VATRegime = regime
			if modelApplies(t, c, "390", tp) {
				t.Errorf("390 should not apply to %s", regime)
			}
			tp.HasGeneralVATActivity = true
			if !modelApplies(t, c, "390", tp) {
				t.Errorf("390 should apply to %s with an additional general-regime activity", regime)
			}
		}
	})

	t.Run("349 exige operaciones intracomunitarias", func(t *testing.T) {
		tp := baseAutonomo()
		if modelApplies(t, c, "349", tp) {
			t.Error("349 should not apply without intracommunity operations")
		}
		tp.HasIntracommunityOperations = true
		if !modelApplies(t, c, "349", tp) {
			t.Error("349 should apply with intracommunity operations")
		}
	})

	t.Run("111 y 190 cubren empleados y retenciones profesionales", func(t *testing.T) {
		for _, code := range []string{"111", "190"} {
			tp := baseAutonomo()
			if modelApplies(t, c, code, tp) {
				t.Errorf("%s should not apply without employees or professional withholding", code)
			}
			tp.HasEmployees = true
			if !modelApplies(t, c, code, tp) {
				t.Errorf("%s should apply with employees", code)
			}
			tp.HasEmployees = false
			tp.WithholdsProfessionalIncome = true
			if !modelApplies(t, c, code, tp) {
				t.Errorf("%s should apply with professional withholding", code)
			}
		}
	})

	t.Run("130 y 131 respetan la exencion del 70 por ciento", func(t *testing.T) {
		tp := baseAutonomo()
		if !modelApplies(t, c, "130", tp) {
			t.Error("130 should apply to direct estimation")
		}
		tp.ExemptFromIRPFFractionalPayment = true
		if modelApplies(t, c, "130", tp) {
			t.Error("130 should not apply when exempt")
		}

		tp = baseAutonomo()
		tp.IRPFMethod = "modulos"
		if !modelApplies(t, c, "131", tp) {
			t.Error("131 should apply to modules")
		}
		tp.ExemptFromIRPFFractionalPayment = true
		if modelApplies(t, c, "131", tp) {
			t.Error("131 should not apply when exempt")
		}
	})

	t.Run("840 solo sociedades", func(t *testing.T) {
		tp := baseAutonomo()
		if modelApplies(t, c, "840", tp) {
			t.Error("840 should not apply to a persona fisica")
		}
		tp.LegalForm = domain.LegalFormSL
		if !modelApplies(t, c, "840", tp) {
			t.Error("840 should be available for a sociedad")
		}
	})
}

func TestModelsWithManualChecks(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, code := range []string{"347", "840", "349"} {
		m, ok := c.Get(code)
		if !ok {
			t.Fatalf("model %s not found", code)
		}
		if strings.TrimSpace(m.ManualCheck) == "" {
			t.Errorf("%s should explain the manual check", code)
		}
	}
}

func TestConditionalModelsAreManual(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, code := range []string{"347", "309"} {
		m, ok := c.Get(code)
		if !ok {
			t.Fatalf("model %s not found", code)
		}
		if m.Auto {
			t.Errorf("%s should be a manual/conditional model", code)
		}
	}

	tp := baseAutonomo()
	tp.UsesSII = true
	if modelApplies(t, c, "347", tp) {
		t.Error("347 should not apply to taxpayers using SII")
	}
}

func TestScheduleDueDaysAndMonthlyConditions(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range c.All() {
		if m.Schedule == nil {
			continue
		}
		if len(m.Schedule.DueDays) > 0 && len(m.Schedule.DueMonths) > 0 &&
			len(m.Schedule.DueDays) != len(m.Schedule.DueMonths) {
			t.Errorf("%s has mismatched due_days and due_months", m.Code)
		}
		if len(m.Schedule.MonthlyDueDays) > 0 && len(m.Schedule.MonthlyDueDays) != 12 {
			t.Errorf("%s has %d monthly_due_days", m.Code, len(m.Schedule.MonthlyDueDays))
		}
		if len(m.Schedule.MonthlyDueMonths) > 0 && len(m.Schedule.MonthlyDueMonths) != 12 {
			t.Errorf("%s has %d monthly_due_months", m.Code, len(m.Schedule.MonthlyDueMonths))
		}
	}

	m303, _ := c.Get("303")
	if !contains(m303.Schedule.MonthlyWhen, "sii") || !contains(m303.Schedule.MonthlyWhen, "monthly_vat") {
		t.Error("303 should switch to monthly with SII or an explicit monthly VAT profile")
	}
	if len(m303.Schedule.DueDays) != 4 || m303.Schedule.DueDays[0] != 30 {
		t.Error("303 4T should be due on 30 January")
	}
	if len(m303.Schedule.MonthlyDueDays) != 12 || m303.Schedule.MonthlyDueDays[1] != 0 {
		t.Error("303 SII monthly_due_days should make the January period due at the end of February")
	}
	m111, _ := c.Get("111")
	if !contains(m111.Schedule.MonthlyWhen, "monthly_withholdings") {
		t.Error("111 should switch to monthly with monthly withholdings")
	}
	m349, _ := c.Get("349")
	if !contains(m349.Schedule.MonthlyWhen, "intracommunity_over_50k") {
		t.Error("349 should switch to monthly above 50,000 EUR")
	}
	if len(m349.Schedule.MonthlyDueMonths) != 12 || m349.Schedule.MonthlyDueMonths[6] != 9 {
		t.Error("349 July period should be due in September")
	}
	m390, _ := c.Get("390")
	if m390.Schedule.DueDay != 30 {
		t.Errorf("390 due_day = %d, want 30", m390.Schedule.DueDay)
	}
	m347, _ := c.Get("347")
	if m347.Schedule.DueDay != 0 {
		t.Errorf("347 due_day = %d, want last day of February", m347.Schedule.DueDay)
	}
}

func TestDocumentationChecklists(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range c.All() {
		if len(m.Documents) == 0 {
			if !m.Obsolete && m.Code != "RETA-COT" {
				t.Errorf("model %s has no documents checklist", m.Code)
			}
			continue
		}
		seen := make(map[string]bool, len(m.Documents))
		for _, raw := range m.Documents {
			doc := strings.TrimSpace(raw)
			if doc == "" {
				t.Errorf("model %s has an empty document entry", m.Code)
				continue
			}
			if seen[doc] {
				t.Errorf("model %s repeats document %q", m.Code, doc)
			}
			seen[doc] = true
			if m.Code != "037" && strings.Contains(doc, "037") {
				t.Errorf("model %s references the obsolete model 037 in its checklist: %q", m.Code, doc)
			}
		}
	}
}
