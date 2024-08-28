package web

import (
	"net/http"

	"github.com/gorilla/mux"
)

func StartWeb(mode string) {
	r := mux.NewRouter();

	if (mode == "server") {
		adminRoutes := r.PathPrefix("/admin").Subrouter()

		adminRoutes.HandleFunc("/health", HandleAdminHealth)
		adminRoutes.HandleFunc("/login", HandleAdminLogin)
		adminRoutes.PathPrefix("/*").Handler(public())
	} else {
		clientRoutes := r.PathPrefix("/").Subrouter()

		clientRoutes.HandleFunc("/health", HandleClientHealth)
	}

	http.ListenAndServe("localhost:9000", r)
}