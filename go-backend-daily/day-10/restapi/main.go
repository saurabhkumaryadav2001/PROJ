package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const maxRequestBody = 1 << 20 // 1 MiB

// Product is the API response/domain shape. The server owns ID and CreatedAt.
type Product struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateProductRequest contains only fields a client is allowed to provide.
type CreateProductRequest struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (req *CreateProductRequest) Validate() error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Price <= 0 {
		return errors.New("price must be greater than zero")
	}
	if req.Price > 1_000_000 {
		return errors.New("price must not exceed 1000000")
	}
	return nil
}

// Store is a concurrency-safe in-memory repository for this learning API.
type Store struct {
	mu       sync.RWMutex
	products map[int]Product
	nextID   int
}

func NewStore() *Store {
	return &Store{products: make(map[int]Product), nextID: 1}
}

func (s *Store) List() []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Product, 0, len(s.products))
	for _, product := range s.products {
		items = append(items, product)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (s *Store) Get(id int) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	product, ok := s.products[id]
	return product, ok
}

func (s *Store) Create(req CreateProductRequest) Product {
	s.mu.Lock()
	defer s.mu.Unlock()

	product := Product{
		ID: s.nextID, Name: req.Name, Price: req.Price,
		CreatedAt: time.Now().UTC(),
	}
	s.products[product.ID] = product
	s.nextID++
	return product
}

func (s *Store) Update(id int, req CreateProductRequest) (Product, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.products[id]; !ok {
		return Product{}, false
	}
	product := s.products[id]
	product.Name = req.Name
	product.Price = req.Price
	s.products[id] = product // preserve ID and CreatedAt
	return product, true
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.products[id]; !ok {
		return false
	}
	delete(s.products, id)
	return true
}

// NewHandler registers method-aware routes available since Go 1.22.
func NewHandler(store *Store) http.Handler {
	mux := http.NewServeMux()
	api := &API{store: store}

	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /products", api.listProducts)
	mux.HandleFunc("POST /products", api.createProduct)
	mux.HandleFunc("GET /products/{id}", api.getProduct)
	mux.HandleFunc("PUT /products/{id}", api.updateProduct)
	mux.HandleFunc("DELETE /products/{id}", api.deleteProduct)

	return mux
}

type API struct {
	store *Store
}

func (api *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) listProducts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.store.List())
}

func (api *API) createProduct(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	product := api.store.Create(req)
	w.Header().Set("Location", fmt.Sprintf("/products/%d", product.ID))
	writeJSON(w, http.StatusCreated, product)
}

func (api *API) getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	product, ok := api.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (api *API) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req CreateProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	product, ok := api.store.Update(id, req)
	if !ok {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (api *API) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !api.store.Delete(id) {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204 has no response body
}

func parseID(raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

// decodeJSON applies a body limit, JSON content-type check, strict field check,
// and a one-document-only rule. Validation is a separate step.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType, _, contentTypeErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentTypeErr != nil || contentType != "application/json" {
		return errors.New("content-type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("body must contain exactly one JSON value")
		}
		return fmt.Errorf("check trailing JSON: %w", err)
	}
	return nil
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body must be at most 1 MiB")
		return
	}
	if strings.HasPrefix(err.Error(), "content-type") {
		writeError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:            NewHandler(NewStore()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("API listening on %s", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			_ = server.Close()
		}
		log.Println("server stopped gracefully")
	}
}
