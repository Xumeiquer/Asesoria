package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/calc"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListDocuments(r.Context(), store.DocumentFilter{
		TaxpayerID: r.URL.Query().Get("taxpayer_id"),
		FilingID:   r.URL.Query().Get("filing_id"),
		Kind:       r.URL.Query().Get("kind"),
		Limit:      intQuery(r, "limit", 200),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "no se pudo leer el formulario: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "falta el fichero")
		return
	}
	defer file.Close()

	relPath, hash, size, err := s.Files.Save(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo guardar el fichero: "+err.Error())
		return
	}

	filename := filepath.Base(header.Filename)
	if filename == "." || filename == "/" {
		filename = "documento"
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		kind = "other"
	}
	doc := domain.Document{
		TaxpayerID:      optionalString(r.FormValue("taxpayer_id")),
		FilingID:        optionalString(r.FormValue("filing_id")),
		Kind:            kind,
		Filename:        filename,
		MIME:            mime,
		Size:            size,
		SHA256:          hash,
		StoragePath:     relPath,
		DocDate:         strings.TrimSpace(r.FormValue("doc_date")),
		CounterpartyNIF: strings.ToUpper(strings.TrimSpace(r.FormValue("counterparty_nif"))),
		Metadata:        map[string]any{"original_name": header.Filename},
	}
	if amount := strings.TrimSpace(r.FormValue("amount")); amount != "" {
		cents, err := calc.ParseEuros(amount)
		if err != nil {
			writeError(w, http.StatusBadRequest, "importe no válido")
			return
		}
		doc.AmountCents = &cents
	}
	if err := s.Store.CreateDocument(r.Context(), &doc); err != nil {
		_ = s.Files.Remove(relPath)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Store.Log(r.Context(), actor(r), "upload", "document", doc.ID, map[string]any{"filename": doc.Filename, "sha256": doc.SHA256})
	writeJSON(w, http.StatusCreated, doc)
}

func (s *Server) handleDownloadDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := s.Store.GetDocument(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	f, err := s.Files.Open(doc.StoragePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "fichero no encontrado en disco")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", doc.MIME)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeHeaderFilename(doc.Filename)+"\"")
	http.ServeContent(w, r, doc.Filename, parseTimeOrZero(doc.CreatedAt), f)
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	doc, err := s.Store.GetDocument(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if err := s.Store.DeleteDocument(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	_ = s.Files.Remove(doc.StoragePath)
	s.Store.Log(r.Context(), actor(r), "delete", "document", id, map[string]any{"filename": doc.Filename})
	w.WriteHeader(http.StatusNoContent)
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func sanitizeHeaderFilename(name string) string {
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	if name == "" {
		return "documento"
	}
	return name
}
