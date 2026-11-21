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

const maxRequestBody int64 = 1 << 20

type API struct{ products *ProductService }

func NewHandler(products *ProductService) http.Handler {
	api := &API{products: products}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("POST /api/v1/products", api.createProduct)
	mux.HandleFunc("GET /api/v1/products", api.listProducts)
	mux.HandleFunc("GET /api/v1/products/{id}", api.getProduct)
	return mux
}

func (api *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) createProduct(w http.ResponseWriter, r *http.Request) {
	if !isJSON(r.Header.Get("Content-Type")) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	var input CreateProductInput
	if err := decodeJSON(w, r, &input); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	product, err := api.products.Create(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/products/%d", product.ID))
	writeJSON(w, http.StatusCreated, product)
}

func (api *API) getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseProductID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "product ID must be a positive integer")
		return
	}
	product, err := api.products.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (api *API) listProducts(w http.ResponseWriter, r *http.Request) {
	limit, offset := 20, 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "offset must be an integer")
			return
		}
	}
	products, err := api.products.List(r.Context(), limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, products)
}

func parseProductID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid product ID")
	}
	return id, nil
}

func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidProduct):
		writeError(w, http.StatusUnprocessableEntity, "product data is invalid")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "product not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
