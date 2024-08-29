//go:build !dev
// +build !dev

package web

import (
	"embed"
	"net/http"
)

//go:embed public
var publicFS embed.FS

func public() http.Handler {
	return http.FileServerFS(publicFS)
}