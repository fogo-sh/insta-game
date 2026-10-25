package launchui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var files embed.FS

func Handler() http.Handler {
	root, _ := fs.Sub(files, "web")
	return http.FileServer(http.FS(root))
}
