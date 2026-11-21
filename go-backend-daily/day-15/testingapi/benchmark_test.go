package main

import (
	"context"
	"testing"
)

func BenchmarkProductServiceCreate(b *testing.B) {
	service := NewProductService(NewMemoryRepository())
	input := CreateProductInput{SKU: "BENCH-1", Name: "Benchmark product", PricePaise: 1000, Stock: 10}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Create(context.Background(), input); err != nil {
			b.Fatal(err)
		}
	}
}
