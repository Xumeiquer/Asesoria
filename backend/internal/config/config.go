// Package config resolves runtime configuration from flags and environment.
package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Addr    string
	DataDir string
	WebDir  string
	Dev     bool
	Demo    bool
}

func Load() Config {
	return Config{
		Addr:    env("ASESORIA_ADDR", "127.0.0.1:8787"),
		DataDir: env("ASESORIA_DATA_DIR", defaultDataDir()),
		WebDir:  env("ASESORIA_WEB_DIR", "web/dist"),
		Dev:     env("ASESORIA_DEV", "") == "1",
		Demo:    env("ASESORIA_DEMO", "") == "1",
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultDataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "asesoria")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "./data"
	}
	return filepath.Join(home, ".local", "share", "asesoria")
}
