package store

import (
	"strconv"
	"strings"
)

func joinAnd(parts []string) string {
	return strings.Join(parts, " AND ")
}

func itoa(n int) string { return strconv.Itoa(n) }
