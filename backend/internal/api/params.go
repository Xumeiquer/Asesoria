package api

import (
	"net/http"
	"strconv"
	"time"
)

func intQuery(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func currentYear() int { return time.Now().Year() }

func currentDate() string { return time.Now().Format("2006-01-02") }

func parseTimeOrZero(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func boolForm(r *http.Request, key string) bool {
	v := r.FormValue(key)
	return v == "1" || v == "true" || v == "on"
}
