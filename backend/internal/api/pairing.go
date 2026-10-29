package api

import (
	"net/http"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/go-chi/chi/v5"
)

type claimInput struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (s *Server) handleClaimPairing(w http.ResponseWriter, r *http.Request) {
	var in claimInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido")
		return
	}
	in.Code = strings.TrimSpace(in.Code)
	if len(in.Code) != 6 {
		writeError(w, http.StatusBadRequest, "código no válido")
		return
	}
	raw, token, err := s.Store.ClaimPairingCode(r.Context(), in.Code, strings.TrimSpace(in.Name))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	s.Store.Log(r.Context(), "extension:"+token.Name, "pair", "token", token.ID, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    raw,
		"token_id": token.ID,
		"name":     token.Name,
	})
}

func (s *Server) handleCreatePairingCode(w http.ResponseWriter, r *http.Request) {
	if !s.requireWeb(w, r) {
		return
	}
	pc, err := s.Store.CreatePairingCode(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pc)
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	if !s.requireWeb(w, r) {
		return
	}
	tokens, err := s.Store.ListTokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	if !s.requireWeb(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.Store.RevokeToken(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "revoke", "token", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireWeb(w http.ResponseWriter, r *http.Request) bool {
	if actor(r) != "web" {
		writeError(w, http.StatusForbidden, "esta operación solo está disponible desde el ERP")
		return false
	}
	return true
}

// --- extension --------------------------------------------------------------

type extensionFiling struct {
	Filing                domain.Filing `json:"filing"`
	TaxpayerName          string        `json:"taxpayer_name"`
	TaxpayerNIF           string        `json:"taxpayer_nif"`
	TaxpayerFiscalYearEnd string        `json:"taxpayer_fiscal_year_end"`
	ModelURL              string        `json:"model_url"`
	Calculator            bool          `json:"calculator"`
}

type extensionDocument struct {
	domain.Document
	DownloadURL string `json:"download_url"`
}

type extensionContext struct {
	Filing    domain.Filing       `json:"filing"`
	Taxpayer  domain.Taxpayer     `json:"taxpayer"`
	Model     catalog.Model       `json:"model"`
	Documents []extensionDocument `json:"documents"`
}

func (s *Server) handleExtensionFilings(w http.ResponseWriter, r *http.Request) {
	statuses := []string{domain.StatusPending, domain.StatusInProgress, domain.StatusReady}
	if raw := r.URL.Query().Get("status"); raw != "" {
		statuses = nil
		for _, part := range strings.Split(raw, ",") {
			if part = strings.TrimSpace(part); part != "" {
				statuses = append(statuses, part)
			}
		}
	}
	filings, err := s.Store.ListFilings(r.Context(), store.FilingFilter{
		TaxpayerID: r.URL.Query().Get("taxpayer_id"),
		Statuses:   statuses,
		Year:       intQuery(r, "year", 0),
		Limit:      intQuery(r, "limit", 100),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	taxpayers, err := s.Store.ListTaxpayers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	byID := make(map[string]domain.Taxpayer, len(taxpayers))
	for _, t := range taxpayers {
		byID[t.ID] = t
	}
	out := make([]extensionFiling, 0, len(filings))
	for _, f := range filings {
		t := byID[f.TaxpayerID]
		model, _ := s.Catalog.Get(f.ModelCode)
		out = append(out, extensionFiling{
			Filing:                f,
			TaxpayerName:          t.Name,
			TaxpayerNIF:           t.NIF,
			TaxpayerFiscalYearEnd: t.FiscalYearEnd,
			ModelURL:              model.URL,
			Calculator:            f.ModelCode == "303" || f.ModelCode == "130",
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleExtensionContext(w http.ResponseWriter, r *http.Request) {
	f, err := s.Store.GetFiling(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	t, err := s.Store.GetTaxpayer(r.Context(), f.TaxpayerID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	model, _ := s.Catalog.Get(f.ModelCode)
	docs, err := s.Store.ListDocuments(r.Context(), store.DocumentFilter{TaxpayerID: f.TaxpayerID, Limit: 200})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]extensionDocument, 0, len(docs))
	for _, d := range docs {
		out = append(out, extensionDocument{
			Document:    d,
			DownloadURL: "/api/documents/" + d.ID + "/download",
		})
	}
	writeJSON(w, http.StatusOK, extensionContext{
		Filing:    f,
		Taxpayer:  t,
		Model:     model,
		Documents: out,
	})
}
