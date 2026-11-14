package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/saurabhkumaryadav2001/go-backend-daily/day-08/shop/internal/catalog"
)

// Handler is the HTTP adapter for catalog operations.
type Handler struct {
	products []catalog.Product
}

// NewHandler receives catalog data from the composition root.
func NewHandler(products []catalog.Product) *Handler {
	return &Handler{products: products}
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
		_ = json.NewEncoder(w).Encode(h.products)
	default:
		http.NotFound(w, r)
	}
}
