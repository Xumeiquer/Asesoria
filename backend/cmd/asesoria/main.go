package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Xumeiquer/asesoria/internal/api"
	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/config"
	"github.com/Xumeiquer/asesoria/internal/db"
	"github.com/Xumeiquer/asesoria/internal/seed"
	"github.com/Xumeiquer/asesoria/internal/storage"
	"github.com/Xumeiquer/asesoria/internal/store"
)

const version = "0.1.0"

func main() {
	cfg := config.Load()
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "dirección de escucha (solo loopback por defecto)")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "directorio de datos (SQLite y documentos)")
	flag.StringVar(&cfg.WebDir, "web-dir", cfg.WebDir, "directorio con el build de la web")
	flag.BoolVar(&cfg.Dev, "dev", cfg.Dev, "modo desarrollo (permite el origen de Vite)")
	flag.BoolVar(&cfg.Demo, "demo", cfg.Demo, "crear datos de demostración si la base está vacía")
	flag.Parse()

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		log.Fatalf("no se pudo crear el directorio de datos: %v", err)
	}

	conn, err := db.Open(filepath.Join(cfg.DataDir, "asesoria.db"))
	if err != nil {
		log.Fatalf("no se pudo abrir la base de datos: %v", err)
	}
	defer conn.Close()

	st := store.New(conn)
	secret, err := st.EnsureSessionSecret(context.Background())
	if err != nil {
		log.Fatalf("no se pudo inicializar la sesión: %v", err)
	}
	cat, err := catalog.Load()
	if err != nil {
		log.Fatalf("no se pudo cargar el catálogo: %v", err)
	}
	files, err := storage.New(filepath.Join(cfg.DataDir, "storage"))
	if err != nil {
		log.Fatalf("no se pudo inicializar el almacenamiento: %v", err)
	}

	if cfg.Demo {
		if err := seed.Demo(context.Background(), st, cat); err != nil {
			log.Fatalf("no se pudieron crear los datos de demostración: %v", err)
		}
	}

	server := api.New(st, cat, files, secret, cfg.WebDir, cfg.DataDir, version, cfg.Dev)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	log.Printf("Asesoría %s · http://%s · datos en %s · catálogo %s", version, cfg.Addr, cfg.DataDir, cat.Version)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("servidor detenido: %v", err)
	}
}
