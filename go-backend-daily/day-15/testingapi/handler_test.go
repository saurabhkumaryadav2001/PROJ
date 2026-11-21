package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return NewHandler(NewProductService(NewMemoryRepository()))
}

func TestCreateProductHTTP(t *testing.T) {
	cases := []struct { name, body, contentType string; wantStatus int }{
		{name: "valid product", body: `{"sku":"KB-1","name":"Keyboard","price_paise":149900,"stock":5}`, contentType: "application/json", wantStatus: http.StatusCreated},
		{name: "malformed JSON", body: `{`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "unknown field", body: `{"sku":"KB-1","name":"Keyboard","price_paise":100,"stock":1,"admin":true}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "invalid domain values", body: `{"sku":"KB-1","name":"Keyboard","price_paise":0,"stock":1}`, contentType: "application/json", wantStatus: http.StatusUnprocessableEntity},
		{name: "wrong content type", body: `{"sku":"KB-1","name":"Keyboard","price_paise":100,"stock":1}`, contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
		{name: "multiple JSON values", body: `{"sku":"KB-1","name":"Keyboard","price_paise":100,"stock":1} {}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(tc.body))
			if tc.contentType != "" { req.Header.Set("Content-Type", tc.contentType) }
			rec := httptest.NewRecorder()
			newTestHandler(t).ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus { t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String()) }
			if tc.wantStatus == http.StatusCreated {
				if got := rec.Header().Get("Location"); got != "/api/v1/products/1" { t.Fatalf("Location = %q", got) }
				var product Product
				if err := json.Unmarshal(rec.Body.Bytes(), &product); err != nil { t.Fatal(err) }
				if product.SKU != "KB-1" || product.PricePaise != 149900 { t.Fatalf("unexpected product: %+v", product) }
			}
		})
	}
}

func TestGetProductHTTP(t *testing.T) {
	handler := newTestHandler(t)
	create := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(
		`{"sku":"M-1","name":"Mouse","price_paise":79900,"stock":3}`,
	))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated { t.Fatalf("create status = %d", created.Code) }

	cases := []struct { path string; wantStatus int }{
		{path: "/api/v1/products/1", wantStatus: http.StatusOK},
		{path: "/api/v1/products/999", wantStatus: http.StatusNotFound},
		{path: "/api/v1/products/not-a-number", wantStatus: http.StatusBadRequest},
		{path: "/api/v1/products/0", wantStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.wantStatus { t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String()) }
		})
	}
}

func TestListProductsPagination(t *testing.T) {
	handler := newTestHandler(t)
	for i := 1; i <= 3; i++ {
		body := fmt.Sprintf(`{"sku":"SKU-%d","name":"Product %d","price_paise":100,"stock":1}`, i, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated { t.Fatalf("create %d status = %d", i, rec.Code) }
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=2&offset=1", nil))
	if rec.Code != http.StatusOK { t.Fatalf("list status = %d", rec.Code) }
	var products []Product
	if err := json.Unmarshal(rec.Body.Bytes(), &products); err != nil { t.Fatal(err) }
	if len(products) != 2 || products[0].SKU != "SKU-2" || products[1].SKU != "SKU-3" {
		t.Fatalf("unexpected page: %+v", products)
	}
}

func TestRequestBodyLimit(t *testing.T) {
	body := `{"sku":"` + strings.Repeat("x", int(maxRequestBody)) + `","name":"Large","price_paise":1,"stock":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge { t.Fatalf("status = %d, want 413; body=%s", rec.Code, rec.Body.String()) }
}

func TestHealthAndMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(t)
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK { t.Fatalf("health status = %d", health.Code) }
	wrongMethod := httptest.NewRecorder()
	handler.ServeHTTP(wrongMethod, httptest.NewRequest(http.MethodDelete, "/healthz", nil))
	if wrongMethod.Code != http.StatusMethodNotAllowed { t.Fatalf("wrong method status = %d, want 405", wrongMethod.Code) }
}

func TestInternalErrorDoesNotLeakDetails(t *testing.T) {
	repo := &fakeRepository{createFn: func(context.Context, CreateProductInput) (Product, error) {
		return Product{}, errors.New("password=super-secret database host=db.internal")
	}}
	handler := NewHandler(NewProductService(repo))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(
		`{"sku":"A","name":"B","price_paise":10,"stock":1}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError { t.Fatalf("status = %d, want 500", rec.Code) }
	if strings.Contains(rec.Body.String(), "super-secret") || strings.Contains(rec.Body.String(), "db.internal") {
		t.Fatalf("internal details leaked: %s", rec.Body.String())
	}
}

func TestHTTPClientAgainstTestServer(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t))
	t.Cleanup(server.Close)
	requestBody := strings.NewReader(`{"sku":"HTTP-1","name":"HTTP test","price_paise":500,"stock":2}`)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/products", requestBody)
	if err != nil { t.Fatal(err) }
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	if err != nil { t.Fatal(err) }
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated { t.Fatalf("status = %d, want 201", response.StatusCode) }
	if response.Header.Get("Location") != "/api/v1/products/1" { t.Fatalf("unexpected Location: %q", response.Header.Get("Location")) }
}

func TestParseProductIDTable(t *testing.T) {
	cases := []struct { name, input string; wantID int64; wantErr bool }{
		{name: "positive ID", input: "42", wantID: 42},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-3", wantErr: true},
		{name: "text", input: "abc", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseProductID(tc.input)
			if (err != nil) != tc.wantErr { t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr) }
			if err == nil && got != tc.wantID { t.Fatalf("ID = %d, want %d", got, tc.wantID) }
		})
	}
}
