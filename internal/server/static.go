package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web
var webFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic("failed to create sub filesystem for web assets: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
