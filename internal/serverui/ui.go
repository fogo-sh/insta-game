package serverui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*.js
var files embed.FS

func Handler() http.Handler {
	root, _ := fs.Sub(files, "dist")
	return http.StripPrefix("/assets/", http.FileServer(http.FS(root)))
}
