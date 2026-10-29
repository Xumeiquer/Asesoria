package api

import (
	"archive/zip"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// handleBackup streams a portable copy of the database and local files. The
// database is copied with SQLite's VACUUM INTO so WAL contents are included
// without stopping the application.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	tmpDir, err := os.MkdirTemp("", "asesoria-backup-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo preparar la copia: "+err.Error())
		return
	}
	defer os.RemoveAll(tmpDir)

	dbCopy := filepath.Join(tmpDir, "asesoria.db")
	escaped := strings.ReplaceAll(dbCopy, "'", "''")
	if _, err := s.Store.DB.ExecContext(r.Context(), "VACUUM INTO '"+escaped+"'"); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo copiar la base de datos: "+err.Error())
		return
	}

	filename := "asesoria-backup-" + time.Now().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")

	zw := zip.NewWriter(w)
	addPath := func(src, name string) error {
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(name)
		header.Method = zip.Deflate
		dst, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(dst, f)
		return err
	}

	if err := addPath(dbCopy, "asesoria.db"); err != nil {
		_ = zw.Close()
		return
	}
	if s.DataDir != "" {
		_ = filepath.WalkDir(s.DataDir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			base := entry.Name()
			if base == "asesoria.db" || base == "asesoria.db-wal" || base == "asesoria.db-shm" {
				return nil
			}
			rel, err := filepath.Rel(s.DataDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return nil
			}
			_ = addPath(path, filepath.Join("datos", rel))
			return nil
		})
	}
	if err := zw.Close(); err != nil {
		// The response is already streaming; there is no JSON status left to send.
		log.Printf("backup: close zip: %v", err)
	}
}
