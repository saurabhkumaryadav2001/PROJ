package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCreateProduct(t *testing.T) {
	valid := CreateProductRequest{SKU: "KB-1", Name: "Keyboard", Price: 100}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	invalid := CreateProductRequest{SKU: "KB-1", Name: "Keyboard", Price: 0}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected price validation error")
	}
}

func TestDecodeOneJSONRejectsUnknownAndTrailing(t *testing.T) {
	var request CreateProductRequest
	if err := decodeOneJSON(strings.NewReader(`{"sku":"A","name":"B","price":1,"unexpected":true}`), &request); err == nil {
		t.Fatal("expected unknown-field error")
	}
	if err := decodeOneJSON(strings.NewReader(`{"sku":"A","name":"B","price":1} {"second":true}`), &request); err == nil {
		t.Fatal("expected trailing-value error")
	}
}

func TestAtomicFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "products.json")
	want := []Product{{ID: 1, SKU: "T-1", Name: "Test", Price: 10}}
	if err := writeProductsAtomic(path, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readProducts(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 || got[0].SKU != "T-1" {
		t.Fatalf("unexpected products: %#v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not found: %v", err)
	}
}
