package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

var (
	ErrInvalidPoolConfig = errors.New("worker count and queue capacity must be positive")
	ErrInvalidItem       = errors.New("quote item contains invalid values")
	ErrAmountOverflow    = errors.New("quote amount exceeds int64 range")
)

type QuoteItem struct {
	SKU            string `json:"sku"`
	UnitPricePaise int64  `json:"unit_price_paise"`
	Quantity       int64  `json:"quantity"`
	ProcessingMS   int    `json:"processing_ms,omitempty"`
}

type Quote struct {
	SKU            string `json:"sku"`
	UnitPricePaise int64  `json:"unit_price_paise"`
	Quantity       int64  `json:"quantity"`
	SubtotalPaise  int64  `json:"subtotal_paise"`
}

type indexedItem struct {
	index int
	item  QuoteItem
}

type quoteOutcome struct {
	index int
	quote Quote
	err   error
}

type QuoteService struct {
	workers   int
	queueSize int
}

func NewQuoteService(workers, queueSize int) (*QuoteService, error) {
	if workers < 1 || queueSize < 1 {
		return nil, ErrInvalidPoolConfig
	}
	if workers > 64 || queueSize > 1024 {
		return nil, errors.New("worker count or queue capacity exceeds configured limit")
	}
	return &QuoteService{workers: workers, queueSize: queueSize}, nil
}

func (s *QuoteService) WorkerCount() int {
	return s.workers
}

// Calculate fans input items out to a bounded worker pool and fans results in.
// Result order matches input order, even when workers finish out of order.
func (s *QuoteService) Calculate(ctx context.Context, items []QuoteItem) ([]Quote, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []Quote{}, nil
	}

	workerCount := min(s.workers, len(items))
	jobs := make(chan indexedItem, s.queueSize)
	results := make(chan quoteOutcome, len(items))

	var workersDone sync.WaitGroup
	workersDone.Add(workerCount)

	worker := func() {
		defer workersDone.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case job, ok := <-jobs:
				if !ok {
					return
				}

				quote, err := calculateOne(ctx, job.item)
				outcome := quoteOutcome{index: job.index, quote: quote, err: err}

				select {
				case results <- outcome:
				case <-ctx.Done():
					return
				}
			}
		}
	}

	for i := 0; i < workerCount; i++ {
		go worker()
	}

	// The producer is the only sender and therefore owns closing jobs.
	go func() {
		defer close(jobs)
		for i, item := range items {
			select {
			case <-ctx.Done():
				return
			case jobs <- indexedItem{index: i, item: item}:
			}
		}
	}()

	// Close results only after every worker has stopped. No sender can then
	// write to a closed channel.
	go func() {
		workersDone.Wait()
		close(results)
	}()

	ordered := make([]Quote, len(items))
	received := 0
	var firstErr error

	for outcome := range results {
		received++
		if outcome.err != nil {
			if firstErr == nil {
				firstErr = outcome.err
			}
			continue
		}
		ordered[outcome.index] = outcome.quote
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	if received != len(items) {
		return nil, fmt.Errorf("worker pool returned %d of %d results", received, len(items))
	}
	return ordered, nil
}

// calculateOne simulates an I/O-bound operation, then performs safe integer math.
func calculateOne(ctx context.Context, item QuoteItem) (Quote, error) {
	if item.ProcessingMS > 0 {
		timer := time.NewTimer(time.Duration(item.ProcessingMS) * time.Millisecond)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return Quote{}, ctx.Err()
		case <-timer.C:
		}
	}

	if item.UnitPricePaise <= 0 || item.Quantity <= 0 {
		return Quote{}, ErrInvalidItem
	}
	if item.Quantity > math.MaxInt64/item.UnitPricePaise {
		return Quote{}, ErrAmountOverflow
	}

	subtotal := item.UnitPricePaise * item.Quantity
	return Quote{
		SKU:            item.SKU,
		UnitPricePaise: item.UnitPricePaise,
		Quantity:       item.Quantity,
		SubtotalPaise:  subtotal,
	}, nil
}
