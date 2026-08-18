package pepperfry_test

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
	"devaraja-anu/decor-scrapper/internal/site/pepperfry"
)

// Ensure Pepperfry implements site.Site interface.
var _ site.Site = (*pepperfry.Pepperfry)(nil)

func TestPepperfry_Metadata(t *testing.T) {
	client := pepperfry.New()
	if client.Name() != "pepperfry" {
		t.Fatalf("expected Name() to be 'pepperfry', got %q", client.Name())
	}

	cats := client.Categories()
	if len(cats) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(cats))
	}

	expectedSlugs := []string{"category/3-seater-sofas", "category/coffee-tables", "category/dining-chairs"}
	expectedNames := []string{"sofas", "coffee_tables", "dining_chairs"}

	for i, c := range cats {
		if c.Slug != expectedSlugs[i] {
			t.Errorf("category[%d] slug mismatch: expected %q, got %q", i, expectedSlugs[i], c.Slug)
		}
		if c.Name != expectedNames[i] {
			t.Errorf("category[%d] name mismatch: expected %q, got %q", i, expectedNames[i], c.Name)
		}
	}
}

func TestPepperfry_ListProducts(t *testing.T) {
	sofasHTML, err := os.ReadFile(filepath.Join("testdata", "category_sofas.html"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/category/3-seater-sofas.html" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(sofasHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := pepperfry.New(
		pepperfry.WithBaseURL(server.URL),
		pepperfry.WithHTTPClient(server.Client()),
	)

	refs, err := client.ListProducts(context.Background(), product.Category{
		Slug: "category/3-seater-sofas",
		Name: "sofas",
	})
	if err != nil {
		t.Fatalf("unexpected error listing products: %v", err)
	}

	if len(refs) != 2 {
		t.Fatalf("expected 2 product refs, got %d", len(refs))
	}

	expectedRefs := []product.ProductRef{
		{
			URL:      server.URL + "/product/prestige-3-seater-fabric-sofa-in-blue-2545615.html",
			Category: "sofas",
		},
		{
			URL:      "https://www.pepperfry.com/product/haven-l-shaped-corner-sofa-in-grey-2545616.html",
			Category: "sofas",
		},
	}

	if !reflect.DeepEqual(refs, expectedRefs) {
		t.Errorf("product refs mismatch.\nExpected: %+v\nGot:      %+v", expectedRefs, refs)
	}
}

func TestPepperfry_FetchProduct_Sofa(t *testing.T) {
	sofaHTML, err := os.ReadFile(filepath.Join("testdata", "product_sofa.html"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/product/prestige-3-seater-fabric-sofa-in-blue-2545615.html" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(sofaHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := pepperfry.New(
		pepperfry.WithBaseURL(server.URL),
		pepperfry.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/product/prestige-3-seater-fabric-sofa-in-blue-2545615.html",
		Category: "sofas",
	})
	if err != nil {
		t.Fatalf("unexpected error fetching product: %v", err)
	}

	if prod.Source != "pepperfry" {
		t.Errorf("expected source 'pepperfry', got %q", prod.Source)
	}
	if prod.ExternalID != "2545615" {
		t.Errorf("expected external ID '2545615', got %q", prod.ExternalID)
	}
	if prod.Name != "Prestige 3-Seater Fabric Sofa in Blue" {
		t.Errorf("expected name 'Prestige 3-Seater Fabric Sofa in Blue', got %q", prod.Name)
	}
	if prod.PriceMinor != 2499900 {
		t.Errorf("expected price_minor 2499900 (paise), got %d", prod.PriceMinor)
	}
	if prod.Currency != "INR" {
		t.Errorf("expected currency 'INR', got %q", prod.Currency)
	}
	if prod.Category != "sofas" {
		t.Errorf("expected category 'sofas', got %q", prod.Category)
	}
	if !prod.InStock {
		t.Errorf("expected in_stock true, got false")
	}
	if prod.ImageURL != "https://cdn.pepperfry.com/img/item/2545615_1.jpg" {
		t.Errorf("expected image URL 'https://cdn.pepperfry.com/...', got %q", prod.ImageURL)
	}
	if len(prod.Styles) != 0 {
		t.Errorf("expected empty styles for pepperfry, got %v", prod.Styles)
	}
	if prod.StableKey() != "pepperfry:2545615" {
		t.Errorf("expected StableKey 'pepperfry:2545615', got %q", prod.StableKey())
	}
}

func TestPepperfry_FetchProduct_CoffeeTable(t *testing.T) {
	tableHTML, err := os.ReadFile(filepath.Join("testdata", "product_coffee_table.html"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/product/solid-teak-wood-coffee-table-1892341.html" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(tableHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := pepperfry.New(
		pepperfry.WithBaseURL(server.URL),
		pepperfry.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/product/solid-teak-wood-coffee-table-1892341.html",
		Category: "coffee_tables",
	})
	if err != nil {
		t.Fatalf("unexpected error fetching product: %v", err)
	}

	if prod.ExternalID != "1892341" {
		t.Errorf("expected external ID '1892341', got %q", prod.ExternalID)
	}
	if prod.PriceMinor != 845000 {
		t.Errorf("expected price_minor 845000, got %d", prod.PriceMinor)
	}
	if prod.Category != "coffee_tables" {
		t.Errorf("expected category 'coffee_tables', got %q", prod.Category)
	}
	if !prod.InStock {
		t.Errorf("expected in_stock true, got false")
	}
	if len(prod.Styles) != 0 {
		t.Errorf("expected empty styles, got %v", prod.Styles)
	}
}

func TestPepperfry_FetchProduct_DiningChair_OutOfStock(t *testing.T) {
	chairHTML, err := os.ReadFile(filepath.Join("testdata", "product_dining_chair.html"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/product/mid-century-walnut-dining-chair-3104529.html" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(chairHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := pepperfry.New(
		pepperfry.WithBaseURL(server.URL),
		pepperfry.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/product/mid-century-walnut-dining-chair-3104529.html",
		Category: "dining_chairs",
	})
	if err != nil {
		t.Fatalf("unexpected error fetching product: %v", err)
	}

	if prod.InStock {
		t.Errorf("expected in_stock false, got true")
	}
	if prod.PriceMinor != 1199900 {
		t.Errorf("expected price_minor 1199900, got %d", prod.PriceMinor)
	}
}

func TestPepperfry_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/furniture-broken.html" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := pepperfry.New(
		pepperfry.WithBaseURL(server.URL),
		pepperfry.WithHTTPClient(server.Client()),
	)

	// Test 500 error on listing
	_, err := client.ListProducts(context.Background(), product.Category{Slug: "furniture-broken", Name: "broken"})
	if err == nil {
		t.Errorf("expected error for 500 listing, got nil")
	}

	// Test 404 on fetch
	_, err = client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/product/nonexistent-9999999.html",
		Category: "sofas",
	})
	if err == nil {
		t.Errorf("expected error for 404 fetch, got nil")
	}
}

func TestPepperfry_ImageExtraction_ExcludesLogo(t *testing.T) {
	// HTML with only logo and icon assets, no genuine product image
	htmlWithLogoOnly := `<!DOCTYPE html>
<html>
<head>
  <meta property="og:image" content="https://ii1.pepperfry.com/assets/w22-pf-logo.svg" />
  <meta name="twitter:image" content="https://ii1.pepperfry.com/assets/logo.png" />
  <meta property="product:retailer_item_id" content="999888" />
  <title>Test Sofa</title>
</head>
<body>
  <h1>Test Sofa</h1>
  <div class="v-product__price"><span class="v-product__price--current">₹15,000</span></div>
  <img src="https://ii1.pepperfry.com/assets/header-icon.svg" />
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlWithLogoOnly))
	}))
	defer server.Close()

	client := pepperfry.New(
		pepperfry.WithBaseURL(server.URL),
		pepperfry.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/product/test-sofa-999888.html",
		Category: "sofas",
	})
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}

	// Must return empty string rather than site logo
	if prod.ImageURL != "" {
		t.Errorf("expected empty image URL when only logo/assets present, got %q", prod.ImageURL)
	}
}

func TestPepperfry_ImageExtraction_JSONLDAndLazyLoad(t *testing.T) {
	// HTML with Schema.org JSON-LD @graph containing Product image and lazy-loaded gallery
	htmlJSONLD := `<!DOCTYPE html>
<html>
<head>
  <script type="application/ld+json">
  {
    "@context": "https://schema.org",
    "@graph": [
      {
        "@type": "WebSite",
        "name": "Pepperfry"
      },
      {
        "@type": "Product",
        "name": "Modern Leather Armchair",
        "image": ["https://ii1.pepperfry.com/media/catalog/product/m/o/modern-leather-armchair.jpg"],
        "offers": {
          "@type": "Offer",
          "price": "18999",
          "priceCurrency": "INR",
          "availability": "https://schema.org/InStock"
        }
      }
    ]
  }
  </script>
  <meta property="product:retailer_item_id" content="777666" />
  <title>Modern Leather Armchair</title>
</head>
<body>
  <h1>Modern Leather Armchair</h1>
  <div class="v-product__price"><span class="v-product__price--current">₹18,999</span></div>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlJSONLD))
	}))
	defer server.Close()

	client := pepperfry.New(
		pepperfry.WithBaseURL(server.URL),
		pepperfry.WithHTTPClient(server.Client()),
	)

	prod, err := client.FetchProduct(context.Background(), product.ProductRef{
		URL:      server.URL + "/product/modern-leather-armchair-777666.html",
		Category: "chairs",
	})
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}

	expectedImg := "https://ii1.pepperfry.com/media/catalog/product/m/o/modern-leather-armchair.jpg"
	if prod.ImageURL != expectedImg {
		t.Errorf("expected JSON-LD product image %q, got %q", expectedImg, prod.ImageURL)
	}
}
