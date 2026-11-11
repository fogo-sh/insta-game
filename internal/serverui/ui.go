package serverui

import (
	"embed"
	"net/http"
)

//go:embed status.js
var files embed.FS

func Handler() http.Handler { return http.FileServer(http.FS(files)) }
