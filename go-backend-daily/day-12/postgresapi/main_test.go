package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRepository struct {
	mu       sync.Mutex
	products map[int64]Product
	nextID   int64
	orders   []Order
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{products: make(map[int64]Product), nextID: 1}
}

func (f *fakeRepository) Ping(context.Context) error { return nil }

func (f *fakeRepository) CreateProduct(_ context.Context, input CreateProductInput) (Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := Product{ID: f.nextID, SKU: input.SKU, Name: input.Name, PricePaise: input.PricePaise, Stock: input.Stock, CreatedAt: time.Now().UTC()}
	f.products[p.ID] = p
	f.nextID++
	return p, nil
}

func (f *fakeRepository) GetProduct(_ context.Context, id int64) (Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return p, nil
}

func (f *fakeRepository) ListProducts(_ context.Context, limit, offset int) ([]Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]Product, 0, len(f.products))
	for id := int64(1); id < f.nextID; id++ {
		if p, ok := f.products[id]; ok {
			items = append(items, p)
		}
	}
	if offset >= len(items) {
		return []Product{}, nil
	}
	items = items[offset:]
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (f *fakeRepository) CreateOrder(_ context.Context, input CreateOrderInput) (Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[input.ProductID]
	if !ok {
		return Order{}, ErrNotFound
	}
	if p.Stock < input.Quantity {
		return Order{}, ErrInsufficientStock
	}
	p.Stock -= input.Quantity
	f.products[p.ID] = p
	o := Order{ID: int64(len(f.orders) + 1), ProductID: p.ID, Quantity: input.Quantity, UnitPricePaise: p.PricePaise, TotalPaise: p.PricePaise * int64(input.Quantity), CreatedAt: time.Now().UTC()}
	f.orders = append(f.orders, o)
	return o, nil
}

func TestCreateProductAndList(t *testing.T) {
	repo := newFakeRepository()
	handler := NewAPI(repo).Handler()
	body := `{"sku":"KB-1","name":"Keyboard","price_paise":149900,"stock":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/api/v1/products/1" {
		t.Fatalf("unexpected Location: %s", rec.Header().Get("Location"))
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=10", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d", listRec.Code)
	}
	if !strings.Contains(listRec.Body.String(), "Keyboard") {
		t.Fatalf("product missing from list: %s", listRec.Body.String())
	}
}

func TestCreateProductRejectsInvalidJSONAndValidation(t *testing.T) {
	handler := NewAPI(newFakeRepository()).Handler()
	cases := []struct {
		name, body string
		want       int
	}{
		{"malformed", `{`, http.StatusBadRequest},
		{"unknown field", `{"sku":"A","name":"B","price_paise":100,"stock":1,"admin":true}`, http.StatusBadRequest},
		{"invalid values", `{"sku":"A","name":"B","price_paise":0,"stock":-1}`, http.StatusUnprocessableEntity},
		{"wrong content type", `{"sku":"A","name":"B","price_paise":100,"stock":1}`, http.StatusUnsupportedMediaType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(tc.body))
			if tc.name != "wrong content type" {
				req.Header.Set("Content-Type", "application/json")
			} else {
				req.Header.Set("Content-Type", "text/plain")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestCreateProductRejectsOversizedBody(t *testing.T) {
	handler := NewAPI(newFakeRepository()).Handler()
	body := `{"sku":"A","name":"` + strings.Repeat("x", maxBodyBytes) + `","price_paise":100,"stock":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d want=%d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestOrderSuccessAndInsufficientStock(t *testing.T) {
	repo := newFakeRepository()
	p, err := repo.CreateProduct(context.Background(), CreateProductInput{SKU: "M-1", Name: "Mouse", PricePaise: 79900, Stock: 2})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPI(repo).Handler()
	body := `{"product_id":` + strconv.FormatInt(p.ID, 10) + `, "quantity":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("order status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("insufficient stock status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetMissingProduct(t *testing.T) {
	handler := NewAPI(newFakeRepository()).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	handler := NewAPI(newFakeRepository()).Handler()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

// This optional integration test needs a disposable PostgreSQL database with migration 001 applied.
func TestPostgresOrderDoesNotOversell(t *testing.T) {
	dsn := strings.TrimSpace(getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx := context.Background()
	db, err := OpenDB(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `TRUNCATE orders, products RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	repo := NewSQLRepository(db)
	p, err := repo.CreateProduct(ctx, CreateProductInput{SKU: "RACE-1", Name: "Race Product", PricePaise: 100, Stock: 5})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.CreateOrder(ctx, CreateOrderInput{ProductID: p.ID, Quantity: 1})
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, ErrInsufficientStock) {
				t.Errorf("unexpected order error: %v", err)
			}
		}()
	}
	wg.Wait()
	got, err := repo.GetProduct(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if successes != 5 || got.Stock != 0 {
		t.Fatalf("successes=%d remaining_stock=%d; want 5 and 0", successes, got.Stock)
	}
}

func getenv(key string) string {
	return os.Getenv(key)
}
