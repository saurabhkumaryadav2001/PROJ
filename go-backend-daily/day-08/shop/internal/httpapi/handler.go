package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/saurabhkumaryadav2001/go-backend-daily/day-08/shop/internal/catalog"
)

// Handler is the HTTP adapter for catalog operations.
type Handler struct{}

// NewHandler constructs the HTTP handler.
func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case "/products":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(catalog.List())
	default:
		http.NotFound(w, r)
	}
}
