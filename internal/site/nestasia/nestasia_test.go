package nestasia_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"devaraja-anu/decor-scrapper/internal/product"
	"devaraja-anu/decor-scrapper/internal/site"
	"devaraja-anu/decor-scrapper/internal/site/nestasia"
)

// Ensure Nestasia implements site.Site interface.
var _ site.Site = (*nestasia.Nestasia)(nil)

func TestNestasia_Metadata(t *testing.T) {
	client := nestasia.New()
	if client.Name() != "nestasia" {
		t.Fatalf("expected Name() to be 'nestasia', got %q", client.Name())
	}

	cats := client.Categories()
	if len(cats) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(cats))
	}

	expectedSlugs := []string{"rugs", "wall-decor", "lamps-lighting"}
	expectedNames := []string{"rugs", "wall_decor", "lighting"}

	for i, c := range cats {
		if c.Slug != expectedSlugs[i] {
			t.Errorf("category[%d] slug mismatch: expected %q, got %q", i, expectedSlugs[i], c.Slug)
		}
		if c.Name != expectedNames[i] {
			t.Errorf("category[%d] name mismatch: expected %q, got %q", i, expectedNames[i], c.Name)
		}
	}
}

func TestNestasia_ListProducts(t *testing.T) {
	rugsData, err := os.ReadFile(filepath.Join("testdata", "collection_rugs.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections/rugs/products.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(rugsData)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := nestasia.New(
		nestasia.WithBaseURL(server.URL),
		nestasia.WithHTTPClient(server.Client()),
	)

	refs, err := client.ListProducts(context.Background(), product.Category{
		Slug: "rugs",
		Name: "rugs",
	})
	if err != nil {
		t.Fatalf("unexpected error listing products: %v", err)
	}

	if len(refs) != 2 {
		t.Fatalf("expected 2 product refs, got %d", len(refs))
	}

	expectedRefs := []product.ProductRef{
		{
			URL:      server.URL + "/products/boho-geometric-handwoven-area-rug",
			Category: "rugs",
		},
		{
			URL:      server.URL + "/products/minimalist-neutral-wool-rug",
			Category: "rugs",
		},
	}

	if !reflect.DeepEqual(refs, expectedRefs) {
		t.Errorf("product refs mismatch.\nExpected: %+v\nGot:      %+v", expectedRefs, refs)
	}
}

func TestNestasia_FetchProduct_Rugs(t *testing.T) {
	productData, err := os.ReadFile(filepath.Join("testdata", "product_rug.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/boho-geometric-handwoven-area-rug.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(productData)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := nestasia.New(
		nestasia.WithBaseURL(server.URL),
		nestasia.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/products/boho-geometric-handwoven-area-rug",
		Category: "rugs",
	})
	if err != nil {
		t.Fatalf("unexpected error fetching product: %v", err)
	}

	if prod.Source != "nestasia" {
		t.Errorf("expected source 'nestasia', got %q", prod.Source)
	}
	if prod.ExternalID != "8127456789012" {
		t.Errorf("expected external ID '8127456789012', got %q", prod.ExternalID)
	}
	if prod.Name != "Boho Geometric Handwoven Area Rug" {
		t.Errorf("expected name 'Boho Geometric Handwoven Area Rug', got %q", prod.Name)
	}
	if prod.PriceMinor != 499900 {
		t.Errorf("expected price_minor 499900, got %d", prod.PriceMinor)
	}
	if prod.Currency != "INR" {
		t.Errorf("expected currency 'INR', got %q", prod.Currency)
	}
	if prod.Category != "rugs" {
		t.Errorf("expected category 'rugs', got %q", prod.Category)
	}
	if !prod.InStock {
		t.Errorf("expected in_stock true, got false")
	}
	if prod.ImageURL != "https://cdn.shopify.com/s/files/1/0012/3456/products/rug1.jpg" {
		t.Errorf("expected image URL 'https://cdn.shopify.com/...', got %q", prod.ImageURL)
	}

	expectedStyles := []string{"bohemian", "scandinavian"}
	if !reflect.DeepEqual(prod.Styles, expectedStyles) {
		t.Errorf("styles mismatch: expected %v, got %v", expectedStyles, prod.Styles)
	}
	if prod.StableKey() != "nestasia:8127456789012" {
		t.Errorf("expected StableKey 'nestasia:8127456789012', got %q", prod.StableKey())
	}
}

func TestNestasia_FetchProduct_WallDecor(t *testing.T) {
	productData, err := os.ReadFile(filepath.Join("testdata", "product_wall_decor.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/sunburst-gold-accent-wall-mirror.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(productData)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := nestasia.New(
		nestasia.WithBaseURL(server.URL),
		nestasia.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/products/sunburst-gold-accent-wall-mirror",
		Category: "wall_decor",
	})
	if err != nil {
		t.Fatalf("unexpected error fetching product: %v", err)
	}

	if prod.ExternalID != "8127456789020" {
		t.Errorf("expected external ID '8127456789020', got %q", prod.ExternalID)
	}
	if prod.PriceMinor != 275050 {
		t.Errorf("expected price_minor 275050 (paise), got %d", prod.PriceMinor)
	}
	if prod.Category != "wall_decor" {
		t.Errorf("expected category 'wall_decor', got %q", prod.Category)
	}

	expectedStyles := []string{"contemporary", "glam"}
	if !reflect.DeepEqual(prod.Styles, expectedStyles) {
		t.Errorf("styles mismatch: expected %v, got %v", expectedStyles, prod.Styles)
	}
}

func TestNestasia_FetchProduct_Lighting_OutOfStock(t *testing.T) {
	productData, err := os.ReadFile(filepath.Join("testdata", "product_lighting.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/nordic-ceramic-table-lamp.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(productData)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := nestasia.New(
		nestasia.WithBaseURL(server.URL),
		nestasia.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/products/nordic-ceramic-table-lamp",
		Category: "lighting",
	})
	if err != nil {
		t.Fatalf("unexpected error fetching product: %v", err)
	}

	if prod.InStock {
		t.Errorf("expected in_stock false, got true")
	}
	if prod.PriceMinor != 389000 {
		t.Errorf("expected price_minor 389000, got %d", prod.PriceMinor)
	}

	expectedStyles := []string{"minimalist", "modern"}
	if !reflect.DeepEqual(prod.Styles, expectedStyles) {
		t.Errorf("styles mismatch: expected %v, got %v", expectedStyles, prod.Styles)
	}
}

func TestNestasia_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections/broken/products.json" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/collections/bad-json/products.json" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not valid json"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := nestasia.New(
		nestasia.WithBaseURL(server.URL),
		nestasia.WithHTTPClient(server.Client()),
	)

	// Test 500 error
	_, err := client.ListProducts(context.Background(), product.Category{Slug: "broken", Name: "broken"})
	if err == nil {
		t.Errorf("expected error for 500 status, got nil")
	}

	// Test malformed JSON
	_, err = client.ListProducts(context.Background(), product.Category{Slug: "bad-json", Name: "bad_json"})
	if err == nil {
		t.Errorf("expected error for malformed json, got nil")
	}

	// Test 404 fetch
	_, err = client.FetchProduct(context.Background(), product.ProductRef{URL: server.URL + "/products/nonexistent", Category: "rugs"})
	if err == nil {
		t.Errorf("expected error for 404 product, got nil")
	}
}
