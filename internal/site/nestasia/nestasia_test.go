package nestasia_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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

func TestNestasia_FetchProduct_JSEndpoint(t *testing.T) {
	jsPayload := `{
		"id": 4722390630509,
		"title": "Petalia Acacia Wood Platter Set",
		"handle": "petalia-acacia-wood-platter-set",
		"available": true,
		"price": 125000,
		"tags": ["dining", "style_Contemporary", "style_Modern", "material_Wood"],
		"images": ["//cdn.shopify.com/s/files/1/2690/0106/products/platter1.jpg"],
		"variants": [
			{"id": 32752913219693, "title": "Default", "available": true, "price": 125000}
		],
		"options": [
			{"name": "Style", "values": ["Contemporary"]}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/petalia-acacia-wood-platter-set.js" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(jsPayload))
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
		URL:      server.URL + "/products/petalia-acacia-wood-platter-set",
		Category: "dining",
	})
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}

	if !prod.InStock {
		t.Errorf("expected in_stock true, got false")
	}
	if prod.PriceMinor != 125000 {
		t.Errorf("expected price_minor 125000, got %d", prod.PriceMinor)
	}
	if prod.ImageURL != "https://cdn.shopify.com/s/files/1/2690/0106/products/platter1.jpg" {
		t.Errorf("expected image URL 'https://cdn.shopify.com/...', got %q", prod.ImageURL)
	}
	expectedStyles := []string{"contemporary", "modern"}
	if !reflect.DeepEqual(prod.Styles, expectedStyles) {
		t.Errorf("expected styles %v, got %v", expectedStyles, prod.Styles)
	}
}

func TestNestasia_StructuredStyles_Extraction(t *testing.T) {
	testCases := []struct {
		name     string
		tags     string
		options  string
		expected []string
	}{
		{
			name:     "Style prefix tags",
			tags:     `["decor", "style_Bohemian", "style_Scandinavian"]`,
			options:  `[]`,
			expected: []string{"bohemian", "scandinavian"},
		},
		{
			name:     "Direct synonyms in tags",
			tags:     `["boho", "nordic", "luxe", "minimal"]`,
			options:  `[]`,
			expected: []string{"bohemian", "glam", "minimalist", "scandinavian"},
		},
		{
			name:     "Structured option fields",
			tags:     `["homeware"]`,
			options:  `[{"name": "Style", "values": ["Mid-Century Modern"]}]`,
			expected: []string{"modern"},
		},
		{
			name:     "Unrelated tags produce no false positives",
			tags:     `["flower", "pot", "style_Tropical", "style_Farmhouse", "decor"]`,
			options:  `[]`,
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonResp := fmt.Sprintf(`{
				"id": 99999,
				"title": "Item",
				"handle": "item",
				"available": true,
				"price": 10000,
				"tags": %s,
				"images": ["//cdn.shopify.com/item.jpg"],
				"options": %s
			}`, tc.tags, tc.options)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(jsonResp))
			}))
			defer server.Close()

			client := nestasia.New(
				nestasia.WithBaseURL(server.URL),
				nestasia.WithHTTPClient(server.Client()),
			)

			if !reflect.DeepEqual(prod.Styles, tc.expected) {
				t.Errorf("styles mismatch for %s: expected %v, got %v", tc.name, tc.expected, prod.Styles)
			}
		})
	}
}

func TestLive_VerifyProductsJSON(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	endpoints := []string{
		"https://nestasia.in/products.json?limit=5",
		"https://nestasia.in/collections/wall-decor/products.json?limit=5",
		"https://nestasia.in/collections/dining/products.json?limit=5",
		"https://nestasia.in/collections/rugs/products.json?limit=5",
	}

	for _, u := range endpoints {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("URL %s error: %v", u, err)
			continue
		}

		var payload struct {
			Products []struct {
				ID     int64  `json:"id"`
				Title  string `json:"title"`
				Handle string `json:"handle"`
			} `json:"products"`
		}

		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()

		t.Logf("URL: %s", u)
		t.Logf("  Status: %d (%s)", resp.StatusCode, resp.Status)
		t.Logf("  Content-Type: %s", resp.Header.Get("Content-Type"))
		t.Logf("  Products Returned: %d", len(payload.Products))
		for i, p := range payload.Products {
			if i >= 2 {
				break
			}
			t.Logf("    - [%d] %s (%s)", p.ID, p.Title, p.Handle)
		}
	}
}
