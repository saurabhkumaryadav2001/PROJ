package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxBodyBytes = 1 << 20 // 1 MiB

var ErrUnsupportedMediaType = errors.New("content-type must be application/json")

type Product struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateProductRequest struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

func (e *ValidationError) Error() string { return "request validation failed" }

func (req *CreateProductRequest) Validate() *ValidationError {
	req.Name = strings.TrimSpace(req.Name)
	var fields []FieldError
	if req.Name == "" {
		fields = append(fields, FieldError{Field: "name", Message: "is required"})
	} else if len([]rune(req.Name)) < 2 || len([]rune(req.Name)) > 80 {
		fields = append(fields, FieldError{Field: "name", Message: "must contain 2 to 80 characters"})
	}
	if req.Price <= 0 {
		fields = append(fields, FieldError{Field: "price", Message: "must be greater than zero"})
	} else if req.Price > 1_000_000 {
		fields = append(fields, FieldError{Field: "price", Message: "must not exceed 1000000"})
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

type ProductStore struct {
	mu       sync.RWMutex
	products []Product
	nextID   int
}

func NewProductStore() *ProductStore {
	return &ProductStore{products: []Product{}, nextID: 1}
}

func (s *ProductStore) List() []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Product, len(s.products))
	copy(result, s.products)
	return result
}

func (s *ProductStore) Create(req CreateProductRequest) Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	product := Product{ID: s.nextID, Name: req.Name, Price: req.Price, CreatedAt: time.Now().UTC()}
	s.nextID++
	s.products = append(s.products, product)
	return product
}

type API struct {
	store  *ProductStore
	logger *log.Logger
}

func NewAPI(store *ProductStore, logger *log.Logger) *API {
	return &API{store: store, logger: logger}
}

func (api *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) listProducts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.store.List())
}

func (api *API) createProduct(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := decodeRequestJSON(w, r, &req); err != nil {
		status := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.Is(err, ErrUnsupportedMediaType):
			status = http.StatusUnsupportedMediaType
		case errors.As(err, &maxBytesErr):
			status = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "request_id": requestIDFrom(r)})
		return
	}
	if validation := req.Validate(); validation != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": "validation failed", "fields": validation.Fields, "request_id": requestIDFrom(r),
		})
		return
	}
	created := api.store.Create(req)
	w.Header().Set("Location", "/api/v1/products/"+strconv.Itoa(created.ID))
	writeJSON(w, http.StatusCreated, created)
}

func (api *API) privateInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "you reached a protected endpoint"})
}

func decodeRequestJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType, _, contentTypeErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentTypeErr != nil || contentType != "application/json" {
		return ErrUnsupportedMediaType
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}
