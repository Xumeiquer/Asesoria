package api

import (
	"errors"
	"net/http"

	"github.com/Xumeiquer/asesoria/internal/library"
)

type libraryInput struct {
	Root    string `json:"root"`
	OCRLang string `json:"ocr_lang"`
}

func (s *Server) handleGetLibrary(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Library.Config(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleUpdateLibrary(w http.ResponseWriter, r *http.Request) {
	var in libraryInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
		return
	}
	cfg, err := s.Library.UpdateConfig(r.Context(), in.Root, in.OCRLang)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Store.Log(r.Context(), actor(r), "update", "library", cfg.Root, map[string]any{"root": cfg.Root, "ocr_lang": cfg.OCRLang})
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleEnsureLibraryTree(w http.ResponseWriter, r *http.Request) {
	res, err := s.Library.EnsureTree(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Store.Log(r.Context(), actor(r), "tree", "library", res.Root, res)
	writeJSON(w, http.StatusOK, res)
}

type scanInput struct {
	TaxpayerID string `json:"taxpayer_id"`
}

func (s *Server) handleScanLibrary(w http.ResponseWriter, r *http.Request) {
	var in scanInput
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, "payload no válido: "+err.Error())
			return
		}
	}
	if in.TaxpayerID == "" {
		in.TaxpayerID = r.URL.Query().Get("taxpayer_id")
	}
	res, err := s.Library.Scan(r.Context(), in.TaxpayerID)
	if errors.Is(err, library.ErrDisabled) {
		writeError(w, http.StatusBadRequest, "configura una carpeta de biblioteca antes de escanear")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
