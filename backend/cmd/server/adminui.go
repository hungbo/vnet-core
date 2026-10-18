package main

import (
	"embed"
	"io/fs"
	"log"
)

// The admin build is copied into embed/ by scripts/build-server.sh (and by the
// Docker build) so a single binary serves both the API and the UI.
//
// The directory is committed with only a placeholder index.html: go:embed fails
// on a missing or empty directory, so the placeholder keeps `go build` working
// for anyone who has not built the admin.
//
//go:embed all:embed
var embeddedAdmin embed.FS

// adminAssets returns the admin build rooted at its own directory, or nil when
// only the placeholder is present so the caller can skip mounting the UI.
func adminAssets() fs.FS {
	sub, err := fs.Sub(embeddedAdmin, "embed")
	if err != nil {
		log.Printf("admin UI not embedded: %v", err)
		return nil
	}
	return sub
}
