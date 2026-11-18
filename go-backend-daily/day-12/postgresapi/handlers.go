package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

const maxBodyBytes = 1 << 20

type API struct {
	repo Repository
}

func NewAPI(repo Repository) *API {
	return &API{repo: repo}
}

func (api *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /api/v1/products", api.listProducts)
	mux.HandleFunc("POST /api/v1/products", api.createProduct)
	mux.HandleFunc("GET /api/v1/products/{id}", api.getProduct)
	mux.HandleFunc("POST /api/v1/orders", api.createOrder)
	return mux
}

func (api *API) health(w http.ResponseWriter, r *http.Request) {
	if err := api.repo.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) createProduct(w http.ResponseWriter, r *http.Request) {
	var input CreateProductInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeDecodeError(w, err)
		return
	}
	input.SKU = strings.TrimSpace(input.SKU)
	input.Name = strings.TrimSpace(input.Name)
	if input.SKU == "" || input.Name == "" || input.PricePaise <= 0 || input.Stock < 0 {
		writeError(w, http.StatusUnprocessableEntity, "sku and name are required; price_paise must be positive; stock cannot be negative")
		return
	}

	product, err := api.repo.CreateProduct(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create product")
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/products/%d", product.ID))
	writeJSON(w, http.StatusCreated, product)
}

func (api *API) getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parsePositiveID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	product, err := api.repo.GetProduct(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not get product")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (api *API) listProducts(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "offset must be zero or greater")
			return
		}
	}
	products, err := api.repo.ListProducts(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list products")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  products,
		"limit":  limit,
		"offset": offset,
	})
}

func (api *API) createOrder(w http.ResponseWriter, r *http.Request) {
	var input CreateOrderInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeDecodeError(w, err)
		return
	}
	if input.ProductID <= 0 || input.Quantity <= 0 || input.Quantity > 10000 {
		writeError(w, http.StatusUnprocessableEntity, "product_id and quantity must be positive; quantity cannot exceed 10000")
		return
	}

	order, err := api.repo.CreateOrder(r.Context(), input)
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "product not found")
		return
	case errors.Is(err, ErrInsufficientStock):
		writeError(w, http.StatusConflict, "insufficient stock")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not create order")
		return
	default:
		writeJSON(w, http.StatusCreated, order)
	}
}

type requestDecodeError struct {
	status  int
	message string
	cause   error
}

func (e *requestDecodeError) Error() string { return e.message }
func (e *requestDecodeError) Unwrap() error { return e.cause }

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &requestDecodeError{status: http.StatusUnsupportedMediaType, message: "content-type must be application/json", cause: err}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return &requestDecodeError{status: http.StatusRequestEntityTooLarge, message: "request body exceeds 1 MiB", cause: err}
		}
		return &requestDecodeError{status: http.StatusBadRequest, message: "invalid JSON request body", cause: err}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return &requestDecodeError{status: http.StatusBadRequest, message: "body must contain exactly one JSON value"}
		}
		return &requestDecodeError{status: http.StatusBadRequest, message: "invalid trailing JSON data", cause: err}
	}
	return nil
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var decodeErr *requestDecodeError
	if errors.As(err, &decodeErr) {
		writeError(w, decodeErr.status, decodeErr.message)
		return
	}
	writeError(w, http.StatusBadRequest, "invalid request body")
}

func parsePositiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
