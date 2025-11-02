package assets

import (
	_ "embed"
	"net/http"
)

//go:embed index.html
var indexHTML []byte

// IndexHandler returns an http.Handler that serves the embedded index page.
func IndexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
}

// IndexBytes exposes the raw embedded index page.
func IndexBytes() []byte {
	return indexHTML
}
