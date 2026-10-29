package api

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/library"
	"github.com/Xumeiquer/asesoria/internal/reta"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/go-chi/chi/v5"
)

type taxpayerInput struct {
	NIF              string   `json:"nif"`
	Name             string   `json:"name"`
	LegalForm        string   `json:"legal_form"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	VATRegime        string   `json:"vat_regime"`
	IRPFMethod       string   `json:"irpf_method"`
	HasEmployees     bool     `json:"has_employees"`
	HasRentals       bool     `json:"has_rentals"`
	EconomicActivity *bool    `json:"economic_activity"`
	CNAEs            []string `json:"cnaes"`
	CNAE             string   `json:"cnae"` // legacy single-code clients
	FiscalYearEnd    string   `json:"fiscal_year_end"`
	IBAN             string   `json:"iban"`
	ActivityStart    string   `json:"activity_start"`

	RetaBaseCents      int64  `json:"reta_base_cents"`
	RetaBaseFrom       string `json:"reta_base_from"`
	RetaNetIncomeCents int64  `json:"reta_net_income_cents"`
	RetaNetIncomeYear  int    `json:"reta_net_income_year"`
}

func (s *Server) handleListTaxpayers(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListTaxpayers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetTaxpayer(w http.ResponseWriter, r *http.Request) {
	t, err := s.Store.GetTaxpayer(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleRetaBrackets(w http.ResponseWriter, r *http.Request) {
	year := intQuery(r, "year", currentYear())
	brackets, ok := reta.Brackets(year)
	if !ok {
		writeError(w, http.StatusBadRequest, "no hay tabla de tramos de RETA para ese año")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"year": year, "brackets": brackets})
}

func (s *Server) handleListRetaHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := s.Store.GetTaxpayer(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	changes, err := s.Store.ListRetaBaseChanges(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, changes)
}

func retaProfileChanged(before, after domain.Taxpayer) bool {
	return before.RetaBaseCents != after.RetaBaseCents ||
		before.RetaBaseFrom != after.RetaBaseFrom ||
		before.RetaNetIncomeCents != after.RetaNetIncomeCents ||
		before.RetaNetIncomeYear != after.RetaNetIncomeYear
}

func taxpayerProfileChanged(before, after domain.Taxpayer) bool {
	return before.LegalForm != after.LegalForm ||
		before.VATRegime != after.VATRegime ||
		before.IRPFMethod != after.IRPFMethod ||
		before.HasEmployees != after.HasEmployees ||
		before.HasRentals != after.HasRentals ||
		before.EconomicActivity != after.EconomicActivity ||
		before.FiscalYearEnd != after.FiscalYearEnd ||
		before.ActivityStart != after.ActivityStart
}

func (s *Server) handleCreateTaxpayer(w http.ResponseWriter, r *http.Request) {
	var in taxpayerInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	t, msg := in.toDomain()
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if t.ActivityStart == "" {
		t.ActivityStart = time.Now().Format("2006-01-02")
	}
	if err := s.Store.CreateTaxpayer(r.Context(), &t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.Store.AddRetaBaseChange(r.Context(), t); err != nil {
		s.Store.Log(r.Context(), actor(r), "reta_history_warning", "taxpayer", t.ID, map[string]any{"error": err.Error()})
	}
	generated := 0
	for _, year := range []int{currentYear(), currentYear() + 1} {
		filings := s.Calendar.Generate(t, year)
		n, err := s.Store.UpsertFilings(r.Context(), filings)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "contribuyente creado, pero falló el calendario: "+err.Error())
			return
		}
		generated += n
	}
	libraryWarning := s.ensureTaxpayerLibrary(r.Context(), t)
	s.Store.Log(r.Context(), actor(r), "create", "taxpayer", t.ID, map[string]any{"nif": t.NIF, "generated_filings": generated})
	resp := map[string]any{"taxpayer": t, "generated_filings": generated}
	if libraryWarning != "" {
		resp["library_warning"] = libraryWarning
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleUpdateTaxpayer(w http.ResponseWriter, r *http.Request) {
	var in taxpayerInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	id := chi.URLParam(r, "id")
	existing, err := s.Store.GetTaxpayer(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	updated, msg := in.toDomain()
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	updated.ID = id
	updated.CreatedAt = existing.CreatedAt
	if updated.ActivityStart == "" {
		updated.ActivityStart = existing.ActivityStart
	}
	if err := s.Store.UpdateTaxpayer(r.Context(), &updated); err != nil {
		handleStoreError(w, err)
		return
	}
	if retaProfileChanged(existing, updated) {
		if err := s.Store.AddRetaBaseChange(r.Context(), updated); err != nil {
			s.Store.Log(r.Context(), actor(r), "reta_history_warning", "taxpayer", id, map[string]any{"error": err.Error()})
		}
	}
	generated := 0
	if taxpayerProfileChanged(existing, updated) {
		for _, year := range []int{currentYear(), currentYear() + 1} {
			n, err := s.Store.UpsertFilings(r.Context(), s.Calendar.Generate(updated, year))
			if err != nil {
				s.Store.Log(r.Context(), actor(r), "calendar_sync_warning", "taxpayer", id, map[string]any{"error": err.Error()})
				break
			}
			generated += n
		}
	}
	if warning := s.ensureTaxpayerLibrary(r.Context(), updated); warning != "" {
		s.Store.Log(r.Context(), actor(r), "library_tree_warning", "taxpayer", id, map[string]any{"error": warning})
	}
	s.Store.Log(r.Context(), actor(r), "update", "taxpayer", id, map[string]any{"generated_filings": generated})
	writeJSON(w, http.StatusOK, map[string]any{"taxpayer": updated, "generated_filings": generated})
}

func (s *Server) handleDeleteTaxpayer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.Store.DeleteTaxpayer(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "delete", "taxpayer", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ensureTaxpayerLibrary creates the library tree for a taxpayer when a root is
// configured. Failures are reported to the caller but never roll back the
// taxpayer itself.
func (s *Server) ensureTaxpayerLibrary(ctx context.Context, t domain.Taxpayer) string {
	root, err := s.Library.Root(ctx)
	if err != nil {
		return err.Error()
	}
	if root == "" {
		return ""
	}
	if _, err := library.EnsureTaxpayerTree(root, t, library.Years()); err != nil {
		return err.Error()
	}
	return ""
}

func (in taxpayerInput) toDomain() (domain.Taxpayer, string) {
	t := domain.Taxpayer{
		NIF:              normalizeNIF(in.NIF),
		Name:             strings.TrimSpace(in.Name),
		LegalForm:        in.LegalForm,
		Email:            strings.TrimSpace(in.Email),
		Phone:            strings.TrimSpace(in.Phone),
		VATRegime:        in.VATRegime,
		IRPFMethod:       in.IRPFMethod,
		HasEmployees:     in.HasEmployees,
		HasRentals:       in.HasRentals,
		EconomicActivity: true,
		CNAEs:            normalizeCNAEs(append(in.CNAEs, in.CNAE)),
		FiscalYearEnd:    strings.TrimSpace(in.FiscalYearEnd),
		IBAN:             normalizeIBAN(in.IBAN),
		ActivityStart:    strings.TrimSpace(in.ActivityStart),

		RetaBaseCents:      in.RetaBaseCents,
		RetaBaseFrom:       strings.TrimSpace(in.RetaBaseFrom),
		RetaNetIncomeCents: in.RetaNetIncomeCents,
		RetaNetIncomeYear:  in.RetaNetIncomeYear,
	}
	if in.EconomicActivity != nil {
		t.EconomicActivity = *in.EconomicActivity
	}
	if t.LegalForm == "" {
		t.LegalForm = domain.LegalFormAutonomo
	}
	if t.LegalForm != domain.LegalFormAutonomo && t.LegalForm != domain.LegalFormSL {
		return t, "forma jurídica no válida (autonomo o sl)"
	}
	if t.NIF == "" {
		return t, "el NIF es obligatorio"
	}
	if !nifPattern.MatchString(t.NIF) {
		return t, "el NIF no tiene un formato válido (ej. 12345678Z o B12345678)"
	}
	if t.Name == "" {
		return t, "el nombre es obligatorio"
	}
	if msg := validateCNAEs(t.CNAEs); msg != "" {
		return t, msg
	}
	if t.ActivityStart != "" {
		if _, err := time.Parse("2006-01-02", t.ActivityStart); err != nil {
			return t, "la fecha de inicio de actividad debe tener formato AAAA-MM-DD"
		}
	}
	if t.IBAN != "" && !validIBAN(t.IBAN) {
		return t, "el IBAN no es válido"
	}
	if t.LegalForm == domain.LegalFormSL {
		t.EconomicActivity = true
		t.RetaBaseCents = 0
		t.RetaBaseFrom = ""
		t.RetaNetIncomeCents = 0
		t.RetaNetIncomeYear = 0
	}
	if t.VATRegime == "" {
		t.VATRegime = "general"
	}
	switch t.VATRegime {
	case "general", "simplificado", "recargo", "exento":
	default:
		return t, "régimen de IVA no válido"
	}
	if t.IRPFMethod == "" {
		t.IRPFMethod = "directa"
	}
	if t.IRPFMethod != "directa" && t.IRPFMethod != "modulos" {
		return t, "método de IRPF no válido"
	}
	if t.FiscalYearEnd == "" {
		t.FiscalYearEnd = "12-31"
	}
	if !fiscalYearEndPattern.MatchString(t.FiscalYearEnd) {
		return t, "el cierre fiscal debe tener formato MM-DD"
	}
	if _, err := time.Parse("01-02", t.FiscalYearEnd); err != nil {
		return t, "el cierre fiscal no es una fecha válida"
	}
	if msg := validateRetaProfile(&t); msg != "" {
		return t, msg
	}
	return t, ""
}

var (
	nifPattern           = regexp.MustCompile(`^(?:\d{8}[A-Z]|[XYZ]\d{7}[A-Z]|[A-Z]\d{7}[A-Z0-9])$`)
	ibanPattern          = regexp.MustCompile(`^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$`)
	cnaePattern          = regexp.MustCompile(`^\d{4}$`)
	fiscalYearEndPattern = regexp.MustCompile(`^(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])$`)
)

func normalizeCNAEs(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func validateCNAEs(values []string) string {
	for _, code := range values {
		if !cnaePattern.MatchString(code) {
			return "cada CNAE debe tener 4 dígitos"
		}
	}
	return ""
}

func normalizeNIF(raw string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "", ".", "").Replace(strings.TrimSpace(raw)))
}

func normalizeIBAN(raw string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
}

func validIBAN(iban string) bool {
	if !ibanPattern.MatchString(iban) {
		return false
	}
	rearranged := iban[4:] + iban[:4]
	rem := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			rem = (rem*10 + int(r-'0')) % 97
		case r >= 'A' && r <= 'Z':
			rem = (rem*100 + int(r-'A') + 10) % 97
		default:
			return false
		}
	}
	return rem == 1
}

func validateRetaProfile(t *domain.Taxpayer) string {
	if t.LegalForm != domain.LegalFormAutonomo {
		return ""
	}
	hasProfile := t.RetaBaseCents != 0 || t.RetaNetIncomeCents != 0
	if !hasProfile {
		// An empty profile is valid. Ignore a stray date/year so the record stays
		// consistent even if an older client sends empty RETA fields.
		t.RetaBaseFrom = ""
		t.RetaNetIncomeYear = 0
		return ""
	}
	if t.RetaBaseCents <= 0 {
		return "indica la base de cotización mensual"
	}
	if t.RetaBaseFrom == "" {
		return "indica desde cuándo es válida la base de cotización"
	}
	if _, err := time.Parse("2006-01-02", t.RetaBaseFrom); err != nil {
		return "la fecha de vigencia debe tener formato AAAA-MM-DD"
	}
	if t.RetaNetIncomeYear == 0 {
		t.RetaNetIncomeYear = currentYear()
	}
	if t.RetaNetIncomeYear < 2023 || t.RetaNetIncomeYear > 2100 {
		return "el año de la previsión de rendimientos no es válido"
	}
	if _, err := reta.ValidateBase(t.RetaNetIncomeYear, t.RetaNetIncomeCents, t.RetaBaseCents); err != nil {
		return err.Error()
	}
	return ""
}

var _ = store.ErrNotFound
