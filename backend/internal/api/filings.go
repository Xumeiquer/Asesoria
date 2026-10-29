package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/calc"
	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	taxpayerID := r.URL.Query().Get("taxpayer_id")
	var taxpayer *domain.Taxpayer
	if taxpayerID != "" {
		t, err := s.Store.GetTaxpayer(r.Context(), taxpayerID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		taxpayer = &t
	}
	type catalogItem struct {
		catalog.Model
		Applicable bool `json:"applicable"`
		Calculator bool `json:"calculator"`
	}
	out := make([]catalogItem, 0, len(s.Catalog.All()))
	for _, m := range s.Catalog.All() {
		applicable := true
		if taxpayer != nil {
			applicable = m.AppliesTo(*taxpayer)
		}
		out = append(out, catalogItem{
			Model:      m,
			Applicable: applicable,
			Calculator: m.Code == "303" || m.Code == "130",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    s.Catalog.Version,
		"updated_at": s.Catalog.UpdatedAt,
		"models":     out,
	})
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	year := intQuery(r, "year", currentYear())
	list, err := s.Store.ListFilings(r.Context(), filingFilterFromQuery(r, year))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"year": year, "filings": list})
}

type generateInput struct {
	TaxpayerID string `json:"taxpayer_id"`
	Year       int    `json:"year"`
}

func (s *Server) handleGenerateCalendar(w http.ResponseWriter, r *http.Request) {
	var in generateInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	if in.Year == 0 {
		in.Year = currentYear()
	}
	if in.Year < 2000 || in.Year > 2100 {
		writeError(w, http.StatusBadRequest, "año no válido")
		return
	}
	taxpayers := make([]domain.Taxpayer, 0)
	if in.TaxpayerID != "" {
		t, err := s.Store.GetTaxpayer(r.Context(), in.TaxpayerID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		taxpayers = append(taxpayers, t)
	} else {
		all, err := s.Store.ListTaxpayers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		taxpayers = all
	}
	inserted := 0
	for _, t := range taxpayers {
		n, err := s.Store.UpsertFilings(r.Context(), s.Calendar.Generate(t, in.Year))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		inserted += n
	}
	s.Store.Log(r.Context(), actor(r), "generate", "calendar", in.TaxpayerID, map[string]any{"year": in.Year, "inserted": inserted})
	writeJSON(w, http.StatusOK, map[string]any{"year": in.Year, "inserted": inserted, "taxpayers": len(taxpayers)})
}

func (s *Server) handleListFilings(w http.ResponseWriter, r *http.Request) {
	year := intQuery(r, "year", 0)
	list, err := s.Store.ListFilings(r.Context(), filingFilterFromQuery(r, year))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func filingFilterFromQuery(r *http.Request, year int) (f store.FilingFilter) {
	f.TaxpayerID = r.URL.Query().Get("taxpayer_id")
	f.ModelCode = r.URL.Query().Get("model_code")
	f.Year = year
	f.From = r.URL.Query().Get("from")
	f.To = r.URL.Query().Get("to")
	if st := r.URL.Query().Get("status"); st != "" {
		for _, part := range strings.Split(st, ",") {
			if part = strings.TrimSpace(part); part != "" {
				f.Statuses = append(f.Statuses, part)
			}
		}
	}
	return f
}

func (s *Server) handleGetFiling(w http.ResponseWriter, r *http.Request) {
	f, err := s.Store.GetFiling(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

type filingUpdate struct {
	Status *string        `json:"status"`
	Notes  *string        `json:"notes"`
	Data   map[string]any `json:"data"`
}

func (s *Server) handleUpdateFiling(w http.ResponseWriter, r *http.Request) {
	var in filingUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	id := chi.URLParam(r, "id")
	f, err := s.Store.GetFiling(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if in.Status != nil {
		if !editableStatus(*in.Status) {
			if *in.Status == domain.StatusPresented {
				writeError(w, http.StatusBadRequest, "usa la acción «Marcar presentado» para registrar la presentación y su justificante")
			} else {
				writeError(w, http.StatusBadRequest, "estado no válido")
			}
			return
		}
		if f.Status == domain.StatusPresented {
			f.PresentedAt = nil
			f.ReceiptDocumentID = nil
		}
		f.Status = *in.Status
	}
	if in.Notes != nil {
		f.Notes = *in.Notes
	}
	if in.Data != nil {
		f.Data = in.Data
	}
	if err := s.Store.UpdateFiling(r.Context(), &f); err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "update", "filing", id, map[string]any{"status": f.Status})
	writeJSON(w, http.StatusOK, f)
}

type bulkFilingStatusInput struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

func (s *Server) handleBulkFilingStatus(w http.ResponseWriter, r *http.Request) {
	var in bulkFilingStatusInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	if len(in.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "selecciona al menos un vencimiento")
		return
	}
	if len(in.IDs) > 500 {
		writeError(w, http.StatusBadRequest, "puedes actualizar como máximo 500 vencimientos a la vez")
		return
	}
	switch in.Status {
	case domain.StatusPending, domain.StatusInProgress, domain.StatusReady,
		domain.StatusPresented, domain.StatusNotApplicable:
	default:
		writeError(w, http.StatusBadRequest, "estado no válido")
		return
	}
	seen := make(map[string]struct{}, len(in.IDs))
	ids := make([]string, 0, len(in.IDs))
	for _, id := range in.IDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "no hay vencimientos válidos en la selección")
		return
	}
	updated, err := s.Store.BulkSetFilingStatus(r.Context(), ids, in.Status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Store.Log(r.Context(), actor(r), "bulk_status", "filing", "", map[string]any{
		"ids": ids, "status": in.Status, "updated": updated,
	})
	writeJSON(w, http.StatusOK, map[string]any{"updated": updated, "status": in.Status})
}

func (s *Server) handleCalculateFiling(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f, err := s.Store.GetFiling(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	taxpayer, err := s.Store.GetTaxpayer(r.Context(), f.TaxpayerID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	// Modelo 130 is cumulative from 1 January to the end of the quarter.
	periodStart := f.PeriodStart
	var previous int64
	if f.ModelCode == "130" {
		periodStart = fmt.Sprintf("%d-01-01", yearOf(f.PeriodStart))
		prior, err := s.Store.PreviousFilings(r.Context(), f.TaxpayerID, "130", yearOf(f.PeriodStart), f.PeriodStart)
		if err == nil {
			for _, p := range prior {
				previous += casillaCents(p.Computed, "07")
			}
		}
	}
	invoices, err := s.Store.InvoicesForPeriod(r.Context(), f.TaxpayerID, periodStart, f.PeriodEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	computed, ok := calc.Calculate(f.ModelCode, invoices, previous)
	if !ok {
		writeError(w, http.StatusConflict, "este modelo todavía no tiene calculadora automática; complétalo manualmente o usa la extensión")
		return
	}
	f.Computed = computed
	if f.Status == domain.StatusPending || f.Status == domain.StatusInProgress {
		f.Status = domain.StatusReady
	}
	if err := s.Store.UpdateFiling(r.Context(), &f); err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "calculate", "filing", id, map[string]any{"model": f.ModelCode, "invoices": len(invoices)})
	writeJSON(w, http.StatusOK, map[string]any{
		"filing":   f,
		"taxpayer": taxpayer,
		"invoices": len(invoices),
	})
}

type presentInput struct {
	ReceiptDocumentID string `json:"receipt_document_id"`
}

func (s *Server) handlePresentFiling(w http.ResponseWriter, r *http.Request) {
	var in presentInput
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
			return
		}
	}
	id := chi.URLParam(r, "id")
	if err := s.Store.MarkPresented(r.Context(), id, in.ReceiptDocumentID); err != nil {
		handleStoreError(w, err)
		return
	}
	f, err := s.Store.GetFiling(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "present", "filing", id, map[string]any{"receipt": in.ReceiptDocumentID})
	writeJSON(w, http.StatusOK, f)
}

// editableStatus lists the states a user can choose directly. "presented" is
// deliberately excluded: it is only reached through the presentation action so
// the receipt (or its absence) is explicit. "late" is no longer a stored state;
// it is derived from the due date.
func editableStatus(status string) bool {
	switch status {
	case domain.StatusPending, domain.StatusInProgress, domain.StatusReady, domain.StatusNotApplicable:
		return true
	}
	return false
}

func casillaCents(computed map[string]any, key string) int64 {
	values, ok := computed["values"].(map[string]any)
	if !ok {
		return 0
	}
	raw, ok := values[key].(string)
	if !ok {
		return 0
	}
	cents, err := calc.ParseEuros(raw)
	if err != nil {
		return 0
	}
	return cents
}

func yearOf(date string) int {
	if len(date) < 4 {
		return currentYear()
	}
	var year int
	for i := 0; i < 4; i++ {
		year = year*10 + int(date[i]-'0')
	}
	return year
}
