package product

import (
	"testing"
	"time"
)

func TestProductStableKey(t *testing.T) {
	tests := []struct {
		name     string
		product  Product
		expected string
	}{
		{
			name: "pepperfry product",
			product: Product{
				Source:     "pepperfry",
				ExternalID: "FR12345",
			},
			expected: "pepperfry:FR12345",
		},
		{
			name: "nestasia product",
			product: Product{
				Source:     "nestasia",
				ExternalID: "7519504793709",
			},
			expected: "nestasia:7519504793709",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.product.StableKey()
			if got != tt.expected {
				t.Errorf("StableKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProductFields(t *testing.T) {
	now := time.Now().UTC()
	p := Product{
		Source:     "nestasia",
		ExternalID: "123",
		URL:        "https://nestasia.in/products/vase",
		Name:       "Ceramic Vase",
		PriceMinor: 149900,
		Currency:   "INR",
		Category:   "vases",
		Styles:     []string{"bohemian", "minimalist"},
		ImageURL:   "https://cdn.shopify.com/img.jpg",
		InStock:    true,
		ScrapedAt:  now,
	}

	if p.PriceMinor != 149900 {
		t.Errorf("expected PriceMinor 149900, got %d", p.PriceMinor)
	}
	if len(p.Styles) != 2 {
		t.Errorf("expected 2 styles, got %d", len(p.Styles))
	}
}
