package launchui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/fogo-sh/insta-game/internal/serverui"
)

//go:embed web/*
var files embed.FS

func Handler() http.Handler {
	root, _ := fs.Sub(files, "web")
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", serverui.Handler())
	mux.Handle("/", http.FileServer(http.FS(root)))
	return mux
}
