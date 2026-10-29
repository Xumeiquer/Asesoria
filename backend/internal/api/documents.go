package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Xumeiquer/asesoria/internal/calc"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/library"
	"github.com/Xumeiquer/asesoria/internal/storage"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListDocuments(r.Context(), store.DocumentFilter{
		TaxpayerID: r.URL.Query().Get("taxpayer_id"),
		FilingID:   r.URL.Query().Get("filing_id"),
		Kind:       r.URL.Query().Get("kind"),
		Review:     r.URL.Query().Get("review"),
		Year:       intQuery(r, "year", 0),
		Quarter:    intQuery(r, "quarter", 0),
		Query:      strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:      intQuery(r, "limit", 500),
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
	taxpayerID := strings.TrimSpace(r.FormValue("taxpayer_id"))
	var taxpayer *domain.Taxpayer
	if taxpayerID != "" {
		t, err := s.Store.GetTaxpayer(r.Context(), taxpayerID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		taxpayer = &t
	}
	var amount *int64
	if raw := strings.TrimSpace(r.FormValue("amount")); raw != "" {
		cents, err := calc.ParseEuros(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "importe no válido")
			return
		}
		amount = &cents
	}
	docDate := strings.TrimSpace(r.FormValue("doc_date"))
	counterparty := strings.ToUpper(strings.TrimSpace(r.FormValue("counterparty_nif")))

	if taxpayer != nil {
		root, err := s.Library.Root(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if root != "" {
			doc, err := s.Library.Ingest(r.Context(), library.UploadInput{
				Taxpayer:        *taxpayer,
				FilingID:        optionalString(r.FormValue("filing_id")),
				Kind:            kind,
				Filename:        filename,
				MIME:            mime,
				DocDate:         docDate,
				AmountCents:     amount,
				CounterpartyNIF: counterparty,
				Reader:          file,
			})
			if errors.Is(err, library.ErrDuplicate) {
				writeError(w, http.StatusConflict, "este documento ya está archivado en la biblioteca")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "no se pudo guardar el fichero: "+err.Error())
				return
			}
			s.Store.Log(r.Context(), actor(r), "upload", "document", doc.ID, map[string]any{"filename": doc.Filename, "sha256": doc.SHA256, "library": true})
			writeJSON(w, http.StatusCreated, doc)
			return
		}
	}

	relPath, hash, size, err := s.Files.Save(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo guardar el fichero: "+err.Error())
		return
	}
	doc := domain.Document{
		TaxpayerID:      optionalString(taxpayerID),
		FilingID:        optionalString(r.FormValue("filing_id")),
		Kind:            kind,
		Filename:        filename,
		MIME:            mime,
		Size:            size,
		SHA256:          hash,
		StoragePath:     relPath,
		DocDate:         docDate,
		AmountCents:     amount,
		CounterpartyNIF: counterparty,
		OCRStatus:       library.OCRNone,
		ReviewStatus:    "ok",
		Metadata:        map[string]any{"original_name": header.Filename},
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
	root := s.Files.Root
	if doc.StorageRoot != "" {
		root = doc.StorageRoot
	}
	f, err := storage.OpenAt(root, doc.StoragePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "fichero no encontrado en disco")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", doc.MIME)
	disposition := "attachment"
	if r.URL.Query().Get("inline") == "1" {
		disposition = "inline"
		// The global policy blocks framing. A same-origin, authenticated preview
		// is the only place where the document needs to be embedded.
		w.Header().Del("X-Frame-Options")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
	}
	w.Header().Set("Content-Disposition", disposition+"; filename=\""+sanitizeHeaderFilename(doc.Filename)+"\"")
	http.ServeContent(w, r, doc.Filename, parseTimeOrZero(doc.CreatedAt), f)
}

type documentUpdate struct {
	TaxpayerID      *string `json:"taxpayer_id"`
	FilingID        *string `json:"filing_id"`
	Kind            *string `json:"kind"`
	DocDate         *string `json:"doc_date"`
	Amount          *string `json:"amount"`
	CounterpartyNIF *string `json:"counterparty_nif"`
	ReviewStatus    *string `json:"review_status"`
}

func (s *Server) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := s.Store.GetDocument(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	var in documentUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	meta := store.DocumentMetadata{
		TaxpayerID:      doc.TaxpayerID,
		FilingID:        doc.FilingID,
		Kind:            doc.Kind,
		DocDate:         doc.DocDate,
		AmountCents:     doc.AmountCents,
		CounterpartyNIF: doc.CounterpartyNIF,
		ReviewStatus:    doc.ReviewStatus,
	}
	if in.TaxpayerID != nil {
		meta.TaxpayerID = optionalString(*in.TaxpayerID)
	}
	if in.FilingID != nil {
		meta.FilingID = optionalString(*in.FilingID)
	}
	if in.Kind != nil && strings.TrimSpace(*in.Kind) != "" {
		meta.Kind = strings.TrimSpace(*in.Kind)
	}
	if in.DocDate != nil && strings.TrimSpace(*in.DocDate) != "" {
		meta.DocDate = strings.TrimSpace(*in.DocDate)
	}
	if in.Amount != nil {
		if strings.TrimSpace(*in.Amount) == "" {
			meta.AmountCents = nil
		} else {
			cents, err := calc.ParseEuros(*in.Amount)
			if err != nil {
				writeError(w, http.StatusBadRequest, "importe no válido")
				return
			}
			meta.AmountCents = &cents
		}
	}
	if in.CounterpartyNIF != nil {
		meta.CounterpartyNIF = strings.ToUpper(strings.TrimSpace(*in.CounterpartyNIF))
	}
	if in.ReviewStatus != nil {
		switch *in.ReviewStatus {
		case "ok", "review":
			meta.ReviewStatus = *in.ReviewStatus
		default:
			writeError(w, http.StatusBadRequest, "estado de revisión no válido")
			return
		}
	}
	if err := s.Store.UpdateDocumentMetadata(r.Context(), doc.ID, meta); err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "update", "document", doc.ID, map[string]any{"review_status": meta.ReviewStatus})
	updated, err := s.Store.GetDocument(r.Context(), doc.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleReprocessDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := s.Store.GetDocument(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if doc.StorageRoot == "" {
		writeError(w, http.StatusBadRequest, "solo se pueden reprocesar documentos de la biblioteca")
		return
	}
	path := filepath.Join(doc.StorageRoot, filepath.FromSlash(doc.LibraryPath))
	text, status, err := library.ExtractFile(r.Context(), path, s.Library.OCRLang(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo extraer el texto: "+err.Error())
		return
	}
	if err := s.Store.UpdateDocumentAnalysis(r.Context(), doc.ID, text, status, doc.ReviewStatus); err != nil {
		handleStoreError(w, err)
		return
	}
	s.Store.Log(r.Context(), actor(r), "reprocess", "document", doc.ID, map[string]any{"ocr_status": status})
	updated, err := s.Store.GetDocument(r.Context(), doc.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
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
	root := s.Files.Root
	if doc.StorageRoot != "" {
		root = doc.StorageRoot
	}
	_ = storage.RemoveAt(root, doc.StoragePath)
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
