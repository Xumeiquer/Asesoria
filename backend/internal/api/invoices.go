package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/go-chi/chi/v5"
)

type invoiceInput struct {
	TaxpayerID       string  `json:"taxpayer_id"`
	Direction        string  `json:"direction"`
	Number           string  `json:"number"`
	InvoiceDate      string  `json:"invoice_date"`
	CounterpartyNIF  string  `json:"counterparty_nif"`
	CounterpartyName string  `json:"counterparty_name"`
	Concept          string  `json:"concept"`
	BaseCents        int64   `json:"base_cents"`
	VATRate          int     `json:"vat_rate"`
	RetentionRate    int     `json:"retention_rate"`
	Category         string  `json:"category"`
	DocumentID       *string `json:"document_id"`
}

func (s *Server) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListInvoices(r.Context(), store.InvoiceFilter{
		TaxpayerID: r.URL.Query().Get("taxpayer_id"),
		Direction:  r.URL.Query().Get("direction"),
		Year:       intQuery(r, "year", 0),
		Limit:      intQuery(r, "limit", 500),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	var in invoiceInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	if _, err := s.Store.GetTaxpayer(r.Context(), in.TaxpayerID); err != nil {
		handleStoreError(w, err)
		return
	}
	if in.Direction != domain.DirectionIssued && in.Direction != domain.DirectionReceived {
		writeError(w, http.StatusBadRequest, "dirección no válida")
		return
	}
	if in.BaseCents < 0 {
		writeError(w, http.StatusBadRequest, "la base no puede ser negativa")
		return
	}
	if in.VATRate < 0 || in.VATRate > 10000 || in.RetentionRate < 0 || in.RetentionRate > 10000 {
		writeError(w, http.StatusBadRequest, "tipo de IVA o retención no válido (usa puntos básicos: 2100 = 21%)")
		return
	}
	if _, err := time.Parse("2006-01-02", in.InvoiceDate); err != nil {
		writeError(w, http.StatusBadRequest, "fecha de factura no válida (YYYY-MM-DD)")
		return
	}
	vat := roundBasisPoints(in.BaseCents, in.VATRate)
	retention := roundBasisPoints(in.BaseCents, in.RetentionRate)
	inv := domain.Invoice{
		TaxpayerID:       in.TaxpayerID,
		Direction:        in.Direction,
		Number:           strings.TrimSpace(in.Number),
		InvoiceDate:      in.InvoiceDate,
		CounterpartyNIF:  strings.ToUpper(strings.TrimSpace(in.CounterpartyNIF)),
		CounterpartyName: strings.TrimSpace(in.CounterpartyName),
		Concept:          strings.TrimSpace(in.Concept),
		BaseCents:        in.BaseCents,
		VATRate:          in.VATRate,
		VATCents:         vat,
		RetentionRate:    in.RetentionRate,
		RetentionCents:   retention,
		TotalCents:       in.BaseCents + vat - retention,
		Category:         strings.TrimSpace(in.Category),
		DocumentID:       in.DocumentID,
	}
	if inv.Number == "" {
		inv.Number = "S/N"
	}
	if err := s.Store.CreateInvoice(r.Context(), &inv); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Store.Log(r.Context(), actor(r), "create", "invoice", inv.ID, map[string]any{"direction": inv.Direction, "total_cents": inv.TotalCents})
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) handleDeleteInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.Store.DeleteInvoice(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "delete", "invoice", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func roundBasisPoints(base int64, rate int) int64 {
	if rate == 0 {
		return 0
	}
	return (base*int64(rate) + 5000) / 10000
}
