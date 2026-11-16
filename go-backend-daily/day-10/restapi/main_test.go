package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductCRUD(t *testing.T) {
	server := httptest.NewServer(NewHandler(NewStore()))
	defer server.Close()

	// Create: POST must return 201 and a Location header.
	response := doRequest(t, server.Client(), http.MethodPost, server.URL+"/products",
		`{"name":"Keyboard","price":1499}`, "application/json")
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201", response.StatusCode)
	}
	location := response.Header.Get("Location")
	if location != "/products/1" {
		t.Fatalf("Location = %q, want /products/1", location)
	}
	created := decodeProduct(t, response)
	if created.ID != 1 || created.Name != "Keyboard" || created.Price != 1499 {
		t.Fatalf("unexpected created product: %#v", created)
	}

	// Read one.
	response = doRequest(t, server.Client(), http.MethodGet, server.URL+location, "", "")
	if response.StatusCode != http.StatusOK || decodeProduct(t, response).Name != "Keyboard" {
		t.Fatal("GET product failed")
	}

	// Update: ID and CreatedAt remain server-owned.
	response = doRequest(t, server.Client(), http.MethodPut, server.URL+location,
		`{"name":"Mechanical Keyboard","price":1999}`, "application/json")
	updated := decodeProduct(t, response)
	if response.StatusCode != http.StatusOK || updated.Name != "Mechanical Keyboard" || updated.ID != 1 {
		t.Fatalf("unexpected update: status=%d product=%#v", response.StatusCode, updated)
	}

	// List.
	response = doRequest(t, server.Client(), http.MethodGet, server.URL+"/products", "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", response.StatusCode)
	}
	var products []Product
	if err := json.NewDecoder(response.Body).Decode(&products); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(products) != 1 || products[0].Name != "Mechanical Keyboard" {
		t.Fatalf("unexpected list: %#v", products)
	}

	// Delete: 204 must have an empty body.
	response = doRequest(t, server.Client(), http.MethodDelete, server.URL+location, "", "")
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if len(body) != 0 {
		t.Fatalf("204 body = %q, want empty", body)
	}

	response = doRequest(t, server.Client(), http.MethodGet, server.URL+location, "", "")
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted product GET status = %d, want 404", response.StatusCode)
	}
}

func TestRequestErrors(t *testing.T) {
	server := httptest.NewServer(NewHandler(NewStore()))
	defer server.Close()

	cases := []struct {
		name, body, contentType string
		wantStatus              int
	}{
		{"malformed JSON", `{`, "application/json", http.StatusBadRequest},
		{"unknown field", `{"name":"Pen","price":10,"admin":true}`, "application/json", http.StatusBadRequest},
		{"invalid price", `{"name":"Pen","price":0}`, "application/json", http.StatusBadRequest},
		{"wrong content type", `{"name":"Pen","price":10}`, "text/plain", http.StatusUnsupportedMediaType},
		{"multiple JSON values", `{"name":"Pen","price":10} {}`, "application/json", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := doRequest(t, server.Client(), http.MethodPost, server.URL+"/products", tc.body, tc.contentType)
			response.Body.Close()
			if response.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.wantStatus)
			}
		})
	}
}

func TestRequestBodyLimit(t *testing.T) {
	server := httptest.NewServer(NewHandler(NewStore()))
	defer server.Close()

	body := `{"name":"` + strings.Repeat("x", maxRequestBody) + `","price":1}`
	response := doRequest(t, server.Client(), http.MethodPost, server.URL+"/products", body, "application/json")
	response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", response.StatusCode)
	}
}

func TestHealthAndMissingProduct(t *testing.T) {
	server := httptest.NewServer(NewHandler(NewStore()))
	defer server.Close()

	response := doRequest(t, server.Client(), http.MethodGet, server.URL+"/healthz", "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", response.StatusCode)
	}
	response.Body.Close()

	response = doRequest(t, server.Client(), http.MethodGet, server.URL+"/products/999", "", "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("missing product status = %d, want 404", response.StatusCode)
	}
	response.Body.Close()
}

func doRequest(t *testing.T, client *http.Client, method, url, body, contentType string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeProduct(t *testing.T, response *http.Response) Product {
	t.Helper()
	defer response.Body.Close()
	var product Product
	if err := json.NewDecoder(response.Body).Decode(&product); err != nil {
		t.Fatal(err)
	}
	return product
}
