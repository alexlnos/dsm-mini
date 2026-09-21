// Package web embeds the built Mini App into the binary.
//
// The files appear here after the frontend is built: `npm run build` in the
// web/ directory puts the result into internal/web/dist. Without them the
// service runs as a plain bot, just without the Mini App.
package web

import (
	"embed"
	"errors"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// ErrNotBuilt means the frontend has not been built.
var ErrNotBuilt = errors.New("the Mini App is not built: run the build in the web/ directory")

// Assets returns a file system with the built app.
func Assets() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, ErrNotBuilt
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, ErrNotBuilt
	}
	return sub, nil
}
