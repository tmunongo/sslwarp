package web

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

// go:embed public/*
// var static embed.FS

func StartWeb(ctx context.Context, mode string) {
	r := mux.NewRouter();

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("no wd")
	}

	log.Println(filepath.Join(wd, "public"))
	r.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir(filepath.Join(os.Getenv("PROJECT_ROOT"), "/public")))))

	if (mode == "server") {
		adminRoutes := r.PathPrefix("/admin").Subrouter()

		adminRoutes.HandleFunc("/health", HandleAdminHealth)
		adminRoutes.HandleFunc("/login", HandleAdminLogin)
	} else {
		clientRoutes := r.PathPrefix("/").Subrouter()

		clientRoutes.HandleFunc("/health", HandleClientHealth)
	}

	log.Println("Now listening on port 9000")
	http.ListenAndServe("localhost:9000", r)
}