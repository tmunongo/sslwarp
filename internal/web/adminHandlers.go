package web

import (
	"context"
	"io"
	"net/http"

	"github.com/tmunongo/sslwarp/views/auth"
)

func HandleAdminHealth(w http.ResponseWriter, r *http.Request) {
	// A very simple health check.
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)

    // In the future we could report back on the status of our DB, or our cache
    // (e.g. Redis) by performing a simple PING, and include them in the response.
    io.WriteString(w, `{"alive": true}`)
}

func HandleAdminLogin(w http.ResponseWriter, r *http.Request) {
	component := auth.Login()

	component.Render(context.Background(), w)
}