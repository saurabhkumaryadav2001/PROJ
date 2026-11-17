package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHandler() http.Handler {
	logger := log.New(io.Discard, "", 0)
	store := NewProductStore()
	api := NewAPI(store, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /api/v1/products", api.listProducts)
	mux.Handle("POST /api/v1/products", BearerAuth("test-token")(http.HandlerFunc(api.createProduct)))
	mux.Handle("GET /api/v1/private", BearerAuth("test-token")(http.HandlerFunc(api.privateInfo)))
	return Chain(mux, RequestID, Logging(logger), Recover(logger), SecurityHeaders)
}

func performRequest(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRequestIDAndSecurityHeaders(t *testing.T) {
	rec := performRequest(testHandler(), http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("request ID header missing")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security header missing")
	}
}

func TestCreateRequiresAuthentication(t *testing.T) {
	rec := performRequest(testHandler(), http.MethodPost, "/api/v1/products", `{"name":"Keyboard","price":100}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestCreateProductAndList(t *testing.T) {
	handler := testHandler()
	created := performRequest(handler, http.MethodPost, "/api/v1/products", `{"name":"Keyboard","price":100}`, "test-token")
	if created.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", created.Code, created.Body.String())
	}
	if created.Header().Get("Location") != "/api/v1/products/1" {
		t.Fatalf("unexpected Location: %s", created.Header().Get("Location"))
	}
	listed := performRequest(handler, http.MethodGet, "/api/v1/products", "", "")
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte("Keyboard")) {
		t.Fatalf("unexpected list response: %s", listed.Body.String())
	}
}

func TestValidationReturns422(t *testing.T) {
	rec := performRequest(testHandler(), http.MethodPost, "/api/v1/products", `{"name":" ","price":-1}`, "test-token")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "fields") {
		t.Fatalf("field errors missing: %s", rec.Body.String())
	}
}

func TestDecodeRejectsUnknownFieldAndMultipleValues(t *testing.T) {
	unknown := performRequest(testHandler(), http.MethodPost, "/api/v1/products", `{"name":"AB","price":1,"admin":true}`, "test-token")
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for unknown field, got %d", unknown.Code)
	}
	multiple := performRequest(testHandler(), http.MethodPost, "/api/v1/products", `{"name":"AB","price":1} {"name":"CD","price":2}`, "test-token")
	if multiple.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for multiple values, got %d", multiple.Code)
	}
}

func TestWrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(`{"name":"AB","price":1}`))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d", rec.Code)
	}
}

func TestRecoverMiddleware(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	boom := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("test panic") })
	handler := Chain(boom, RequestID, Recover(logger))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestOversizedBodyReturns413(t *testing.T) {
	body := `{"name":"` + strings.Repeat("A", maxBodyBytes) + `","price":1}`
	rec := performRequest(testHandler(), http.MethodPost, "/api/v1/products", body, "test-token")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", rec.Code)
	}
}

func TestMiddlewareOrder(t *testing.T) {
	first := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "A-before|")
			next.ServeHTTP(w, r)
			_, _ = io.WriteString(w, "|A-after")
		})
	}
	second := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "B-before|")
			next.ServeHTTP(w, r)
			_, _ = io.WriteString(w, "|B-after")
		})
	}
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "handler") })
	rec := httptest.NewRecorder()
	Chain(final, first, second).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	want := "A-before|B-before|handler|B-after|A-after"
	if rec.Body.String() != want {
		t.Fatalf("want %q, got %q", want, rec.Body.String())
	}
}

func TestRequestIDsAreUnique(t *testing.T) {
	handler := testHandler()
	one := performRequest(handler, http.MethodGet, "/healthz", "", "").Header().Get("X-Request-ID")
	two := performRequest(handler, http.MethodGet, "/healthz", "", "").Header().Get("X-Request-ID")
	if one == "" || two == "" || one == two {
		t.Fatalf("expected distinct non-empty IDs, got %q and %q", one, two)
	}
}
