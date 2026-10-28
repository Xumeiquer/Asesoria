// Package webapp embeds the compiled Vue application so the backend can be
// distributed as a single Go binary.
package webapp

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the embedded web build. The dist directory always exists because
// .keep is tracked in the repository; until the frontend is built it contains
// no index.html and the API serves a helpful message instead.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic("webapp: embedded dist directory is missing: " + err.Error())
	}
	return sub
}

// HasIndex reports whether a compiled SPA is embedded in the binary.
func HasIndex() bool {
	_, err := fs.Stat(FS(), "index.html")
	return err == nil
}
