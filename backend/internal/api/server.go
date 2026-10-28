// Package api exposes the local HTTP API consumed by the Vue app and the
// browser extension. It binds to loopback by default.
package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Xumeiquer/asesoria/internal/calendar"
	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/library"
	"github.com/Xumeiquer/asesoria/internal/storage"
	"github.com/Xumeiquer/asesoria/internal/store"
	"github.com/Xumeiquer/asesoria/internal/webapp"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const sessionCookie = "asesoria_session"

type Server struct {
	Store    *store.Store
	Catalog  *catalog.Catalog
	Calendar *calendar.Generator
	Files    *storage.Storage
	Library  *library.Service
	Secret   []byte
	WebDir   string
	DataDir  string
	Dev      bool
	Version  string
}

func New(st *store.Store, cat *catalog.Catalog, files *storage.Storage, secret, webDir, dataDir, version string, dev bool) *Server {
	return &Server{
		Store:    st,
		Catalog:  cat,
		Calendar: calendar.New(cat),
		Files:    files,
		Library:  library.New(st),
		Secret:   []byte(secret),
		WebDir:   webDir,
		DataDir:  dataDir,
		Dev:      dev,
		Version:  version,
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.securityHeaders)
	r.Use(s.checkHost)
	r.Use(s.checkOrigin)
	r.Use(s.cors)

	r.Route("/api", func(api chi.Router) {
		api.Get("/health", s.handleHealth)
		api.Get("/session", s.handleSession)
		api.Post("/pairing/claim", s.handleClaimPairing)

		api.Group(func(pr chi.Router) {
			pr.Use(s.authenticate)
			pr.Use(s.extensionReadOnly)

			pr.Get("/meta", s.handleMeta)

			pr.Get("/dashboard", s.handleDashboard)

			pr.Get("/taxpayers", s.handleListTaxpayers)
			pr.Post("/taxpayers", s.handleCreateTaxpayer)
			pr.Get("/taxpayers/{id}", s.handleGetTaxpayer)
			pr.Put("/taxpayers/{id}", s.handleUpdateTaxpayer)
			pr.Delete("/taxpayers/{id}", s.handleDeleteTaxpayer)
			pr.Get("/taxpayers/{id}/reta-history", s.handleListRetaHistory)

			pr.Get("/reta/brackets", s.handleRetaBrackets)

			pr.Get("/catalog", s.handleCatalog)
			pr.Get("/calendar", s.handleCalendar)
			pr.Post("/calendar/generate", s.handleGenerateCalendar)

			pr.Get("/filings", s.handleListFilings)
			pr.Get("/filings/{id}", s.handleGetFiling)
			pr.Patch("/filings/{id}", s.handleUpdateFiling)
			pr.Post("/filings/{id}/calculate", s.handleCalculateFiling)
			pr.Post("/filings/{id}/present", s.handlePresentFiling)

			pr.Get("/documents", s.handleListDocuments)
			pr.Post("/documents", s.handleUploadDocument)
			pr.Get("/documents/{id}/download", s.handleDownloadDocument)
			pr.Patch("/documents/{id}", s.handleUpdateDocument)
			pr.Post("/documents/{id}/reprocess", s.handleReprocessDocument)
			pr.Delete("/documents/{id}", s.handleDeleteDocument)

			pr.Get("/library", s.handleGetLibrary)
			pr.Put("/library", s.handleUpdateLibrary)
			pr.Post("/library/tree", s.handleEnsureLibraryTree)
			pr.Post("/library/scan", s.handleScanLibrary)

			pr.Get("/invoices", s.handleListInvoices)
			pr.Post("/invoices", s.handleCreateInvoice)
			pr.Delete("/invoices/{id}", s.handleDeleteInvoice)

			pr.Post("/pairing/codes", s.handleCreatePairingCode)
			pr.Get("/pairing/tokens", s.handleListTokens)
			pr.Delete("/pairing/tokens/{id}", s.handleRevokeToken)

			pr.Get("/extension/filings", s.handleExtensionFilings)
			pr.Get("/extension/context/{id}", s.handleExtensionContext)
		})

		api.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "endpoint no encontrado")
		})
		api.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "método no permitido")
		})
	})

	r.NotFound(s.handleWeb)
	return r
}

// --- middleware -------------------------------------------------------------

func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			writeError(w, http.StatusForbidden, "host no permitido")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !s.originAllowed(origin, r.Host) {
			writeError(w, http.StatusForbidden, "origen no permitido")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin, host string) bool {
	if strings.HasPrefix(origin, "chrome-extension://") || strings.HasPrefix(origin, "moz-extension://") {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == host {
		return true
	}
	if s.Dev {
		return u.Host == "localhost:5173" || u.Host == "127.0.0.1:5173"
	}
	return false
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := ""
		if c, err := r.Cookie(sessionCookie); err == nil && s.validSession(c.Value) {
			actor = "web"
		}
		if actor == "" {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
				if token, ok := s.Store.ValidateToken(r.Context(), raw); ok {
					actor = "extension:" + token.Name
				}
			}
		}
		if actor == "" {
			writeError(w, http.StatusUnauthorized, "sesión no válida")
			return
		}
		ctx := context.WithValue(r.Context(), actorKey{}, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type actorKey struct{}

func actor(r *http.Request) string {
	if v, ok := r.Context().Value(actorKey{}).(string); ok {
		return v
	}
	return "unknown"
}

// extensionReadOnly keeps API tokens paired with the browser extension from
// changing ERP data and limits them to the endpoints the extension needs:
// extension listings/context and document downloads. Any write or broader read
// must happen in the web session.
func (s *Server) extensionReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(actor(r), "extension:") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusForbidden, "la extensión solo tiene acceso de lectura")
			return
		}
		path := r.URL.Path
		allowed := path == "/api/extension/filings" ||
			strings.HasPrefix(path, "/api/extension/context/") ||
			(strings.HasPrefix(path, "/api/documents/") && strings.HasSuffix(path, "/download"))
		if !allowed {
			writeError(w, http.StatusForbidden, "recurso no disponible para la extensión")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) sessionValue() string {
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte("asesoria-owner"))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) validSession(value string) bool {
	return hmac.Equal([]byte(value), []byte(s.sessionValue()))
}

// --- helpers ----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// --- static web -------------------------------------------------------------

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusNotFound, "no encontrado")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.sessionValue(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60,
	})

	clean := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if clean == "." {
		clean = ""
	}
	if s.serveDiskWeb(w, r, clean) {
		return
	}
	s.serveEmbeddedWeb(w, r, clean)
}

// serveDiskWeb serves an external web build, which is useful during development
// and for the Vite workflow. It returns false when no external build is
// available so the embedded SPA can take over.
func (s *Server) serveDiskWeb(w http.ResponseWriter, r *http.Request, clean string) bool {
	if s.WebDir == "" {
		return false
	}
	if clean != "" {
		filePath := filepath.Join(s.WebDir, filepath.FromSlash(clean))
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, filePath)
			return true
		}
	}
	index := filepath.Join(s.WebDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		return false
	}
	http.ServeFile(w, r, index)
	return true
}

func (s *Server) serveEmbeddedWeb(w http.ResponseWriter, r *http.Request, clean string) {
	webFS := webapp.FS()
	if clean != "" && !strings.HasPrefix(path.Base(clean), ".") {
		if info, err := fs.Stat(webFS, clean); err == nil && !info.IsDir() {
			http.FileServer(http.FS(webFS)).ServeHTTP(w, r)
			return
		}
	}
	index, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("La interfaz web no está embebida en este binario. Compila con `make build-backend`.\n"))
		return
	}
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
}

// handleSession sets the local session cookie. It is used by the Vite dev server
// and by the first load of the SPA.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.sessionValue(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": s.Version,
		"catalog": s.Catalog.Version,
	})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	tokens, _ := s.Store.ListTokens(r.Context())
	calculators := []string{"303", "130"}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":            s.Version,
		"catalog_version":    s.Catalog.Version,
		"catalog_updated_at": s.Catalog.UpdatedAt,
		"data_dir":           s.DataDir,
		"paired_tokens":      len(tokens),
		"calculators":        calculators,
		"dev":                s.Dev,
	})
}

// --- dashboard --------------------------------------------------------------

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	year := intQuery(r, "year", 0)
	if year == 0 {
		year = currentYear()
	}
	d, err := s.Store.Dashboard(r.Context(), r.URL.Query().Get("taxpayer_id"), year)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// --- common errors ----------------------------------------------------------

func handleStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no encontrado")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
