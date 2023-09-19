package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

type TmplData struct {
	ServiceName string
	BuildData   string
	ShowMetrics bool
}

func RootHandler(data any) http.Handler {
	rootFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	// return http.FileServer(http.FS(rootFS))
	tmplRootFS, err := NewTmplFS(rootFS, data)
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(tmplRootFS))
}
