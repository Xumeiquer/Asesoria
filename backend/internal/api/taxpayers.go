package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/library"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/go-chi/chi/v5"
)

type taxpayerInput struct {
	NIF              string `json:"nif"`
	Name             string `json:"name"`
	LegalForm        string `json:"legal_form"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	VATRegime        string `json:"vat_regime"`
	IRPFMethod       string `json:"irpf_method"`
	HasEmployees     bool   `json:"has_employees"`
	HasRentals       bool   `json:"has_rentals"`
	EconomicActivity *bool  `json:"economic_activity"`
	CNAE             string `json:"cnae"`
	FiscalYearEnd    string `json:"fiscal_year_end"`
	IBAN             string `json:"iban"`
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
	if err := s.Store.CreateTaxpayer(r.Context(), &t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
	if err := s.Store.UpdateTaxpayer(r.Context(), &updated); err != nil {
		handleStoreError(w, err)
		return
	}
	if warning := s.ensureTaxpayerLibrary(r.Context(), updated); warning != "" {
		s.Store.Log(r.Context(), actor(r), "library_tree_warning", "taxpayer", id, map[string]any{"error": warning})
	}
	s.Store.Log(r.Context(), actor(r), "update", "taxpayer", id, nil)
	writeJSON(w, http.StatusOK, updated)
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
		NIF:              strings.ToUpper(strings.TrimSpace(in.NIF)),
		Name:             strings.TrimSpace(in.Name),
		LegalForm:        in.LegalForm,
		Email:            strings.TrimSpace(in.Email),
		Phone:            strings.TrimSpace(in.Phone),
		VATRegime:        in.VATRegime,
		IRPFMethod:       in.IRPFMethod,
		HasEmployees:     in.HasEmployees,
		HasRentals:       in.HasRentals,
		EconomicActivity: true,
		CNAE:             strings.TrimSpace(in.CNAE),
		FiscalYearEnd:    in.FiscalYearEnd,
		IBAN:             strings.TrimSpace(in.IBAN),
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
	if t.Name == "" {
		return t, "el nombre es obligatorio"
	}
	if t.LegalForm == domain.LegalFormSL {
		t.EconomicActivity = true
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
	return t, ""
}

var _ = store.ErrNotFound
