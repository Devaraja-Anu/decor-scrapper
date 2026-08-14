package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"devaraja-anu/decor-scrapper/internal/pipeline"
	"devaraja-anu/decor-scrapper/internal/product"
	"devaraja-anu/decor-scrapper/internal/site"
	"devaraja-anu/decor-scrapper/internal/store"
)

type mockSite struct {
	name       string
	categories []product.Category
	listFunc   func(ctx context.Context, cat product.Category) ([]product.ProductRef, error)
	fetchFunc  func(ctx context.Context, ref product.ProductRef) (product.Product, error)
}

func (m *mockSite) Name() string {
	return m.name
}

func (m *mockSite) Categories() []product.Category {
	return m.categories
}

func (m *mockSite) ListProducts(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, cat)
	}
	return nil, nil
}

func (m *mockSite) FetchProduct(ctx context.Context, ref product.ProductRef) (product.Product, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, ref)
	}
	return product.Product{
		Source:     m.name,
		ExternalID: "mock-1",
		Name:       "Mock Product",
		Category:   ref.Category,
		PriceMinor: 100000,
		Currency:   "INR",
		InStock:    true,
	}, nil
}

func TestPipeline_EndToEndWithStore(t *testing.T) {
	tmpDir := t.TempDir()
	catPath := filepath.Join(tmpDir, "catalog.json")
	fileStore := store.NewJSONFileStore(catPath)

	site1 := &mockSite{
		name: "site1",
		categories: []product.Category{
			{Slug: "cat1", Name: "category1"},
		},
		listFunc: func(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
			return []product.ProductRef{
				{URL: "https://site1.com/item1", Category: cat.Name},
				{URL: "https://site1.com/item2", Category: cat.Name},
			}, nil
		},
		fetchFunc: func(ctx context.Context, ref product.ProductRef) (product.Product, error) {
			id := "1"
			if ref.URL == "https://site1.com/item2" {
				id = "2"
			}
			return product.Product{
				Source:     "site1",
				ExternalID: id,
				URL:        ref.URL,
				Name:       "Product " + id,
				PriceMinor: 50000,
				Currency:   "INR",
				Category:   ref.Category,
				InStock:    true,
			}, nil
		},
	}

	site2 := &mockSite{
		name: "site2",
		categories: []product.Category{
			{Slug: "cat2", Name: "category2"},
		},
		listFunc: func(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
			return []product.ProductRef{
				{URL: "https://site2.com/itemA", Category: cat.Name},
			}, nil
		},
		fetchFunc: func(ctx context.Context, ref product.ProductRef) (product.Product, error) {
			return product.Product{
				Source:     "site2",
				ExternalID: "A",
				URL:        ref.URL,
				Name:       "Product A",
				PriceMinor: 75000,
				Currency:   "INR",
				Category:   ref.Category,
				InStock:    true,
			}, nil
		},
	}

	p := pipeline.New(fileStore, []site.Site{site1, site2}, pipeline.Config{
		Workers:   2,
		RateLimit: rate.Limit(100),
		Burst:     10,
	})

	products, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	if len(products) != 3 {
		t.Fatalf("expected 3 products returned, got %d", len(products))
	}

	// Verify catalog file was written and can be loaded
	savedProducts, err := fileStore.Load(context.Background())
	if err != nil {
		t.Fatalf("loading saved catalog: %v", err)
	}
	if len(savedProducts) != 3 {
		t.Fatalf("expected 3 products in store, got %d", len(savedProducts))
	}
}

func TestPipeline_Concurrency(t *testing.T) {
	const itemCount = 6
	const fetchDelay = 50 * time.Millisecond

	siteObj := &mockSite{
		name: "fastsite",
		categories: []product.Category{
			{Slug: "c", Name: "c"},
		},
		listFunc: func(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
			refs := make([]product.ProductRef, itemCount)
			for i := 0; i < itemCount; i++ {
				refs[i] = product.ProductRef{URL: fmt.Sprintf("https://test.com/item/%d", i), Category: cat.Name}
			}
			return refs, nil
		},
		fetchFunc: func(ctx context.Context, ref product.ProductRef) (product.Product, error) {
			time.Sleep(fetchDelay)
			return product.Product{
				Source:     "fastsite",
				ExternalID: ref.URL,
				Name:       "Item",
				PriceMinor: 1000,
				Currency:   "INR",
			}, nil
		},
	}

	p := pipeline.New(nil, []site.Site{siteObj}, pipeline.Config{
		Workers:   6,
		RateLimit: rate.Limit(1000),
		Burst:     10,
	})

	start := time.Now()
	prods, err := p.Run(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prods) != itemCount {
		t.Fatalf("expected %d items, got %d", itemCount, len(prods))
	}

	// 6 workers running 6 items with 50ms delay should finish in ~50-150ms, much faster than sequential 300ms
	if elapsed >= 300*time.Millisecond {
		t.Errorf("pipeline took %v, expected concurrent execution < 300ms", elapsed)
	}
}

func TestPipeline_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	siteObj := &mockSite{
		name: "cancelsite",
		categories: []product.Category{
			{Slug: "c", Name: "c"},
		},
		listFunc: func(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
			return []product.ProductRef{
				{URL: "https://test.com/1", Category: "c"},
				{URL: "https://test.com/2", Category: "c"},
			}, nil
		},
		fetchFunc: func(ctx context.Context, ref product.ProductRef) (product.Product, error) {
			cancel() // cancel context mid-flight
			return product.Product{}, errors.New("aborted")
		},
	}

	p := pipeline.New(nil, []site.Site{siteObj}, pipeline.Config{
		Workers:   1,
		RateLimit: rate.Limit(100),
		Burst:     1,
	})

	_, err := p.Run(ctx)
	if err == nil {
		t.Errorf("expected context cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestPipeline_ErrorResilience(t *testing.T) {
	var errCount int32
	var mu sync.Mutex
	var reportedErrors []string

	siteObj := &mockSite{
		name: "errsite",
		categories: []product.Category{
			{Slug: "c", Name: "c"},
		},
		listFunc: func(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
			return []product.ProductRef{
				{URL: "https://test.com/good", Category: "c"},
				{URL: "https://test.com/bad", Category: "c"},
			}, nil
		},
		fetchFunc: func(ctx context.Context, ref product.ProductRef) (product.Product, error) {
			if ref.URL == "https://test.com/bad" {
				return product.Product{}, errors.New("network timeout")
			}
			return product.Product{
				Source:     "errsite",
				ExternalID: "good-1",
				Name:       "Good Item",
				PriceMinor: 1000,
				Currency:   "INR",
			}, nil
		},
	}

	p := pipeline.New(nil, []site.Site{siteObj}, pipeline.Config{
		Workers:   2,
		RateLimit: rate.Limit(100),
		Burst:     5,
		OnFetchError: func(siteName string, ref product.ProductRef, err error) {
			atomic.AddInt32(&errCount, 1)
			mu.Lock()
			reportedErrors = append(reportedErrors, err.Error())
			mu.Unlock()
		},
	})

	products, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected pipeline error: %v", err)
	}

	if len(products) != 1 {
		t.Fatalf("expected 1 successful product, got %d", len(products))
	}
	if products[0].ExternalID != "good-1" {
		t.Errorf("expected good-1 item, got %q", products[0].ExternalID)
	}
	if atomic.LoadInt32(&errCount) != 1 {
		t.Errorf("expected 1 error reported, got %d", errCount)
	}
}

func TestPipeline_EmptyListing(t *testing.T) {
	siteObj := &mockSite{
		name: "emptysite",
		categories: []product.Category{
			{Slug: "empty", Name: "empty"},
		},
		listFunc: func(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
			return []product.ProductRef{}, nil
		},
	}

	p := pipeline.New(nil, []site.Site{siteObj}, pipeline.Config{})
	prods, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prods) != 0 {
		t.Errorf("expected 0 products, got %d", len(prods))
	}
}
