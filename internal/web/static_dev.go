//+build dev
//go:build dev
// +build dev

package web

import (
	"fmt"
	"net/http"
	"os"
)

func public() http.Handler {	
	fmt.Println("building static files for development")
	return http.StripPrefix("/public/", http.FileServerFS(os.DirFS("public")))
}