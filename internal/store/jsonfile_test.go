package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devaraja-anu/decor-scrapper/internal/product"
)

func TestJSONFileStore_LoadNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "catalog.json")

	store := NewJSONFileStore(filePath)
	products, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for non-existent file, got %v", err)
	}
	if len(products) != 0 {
		t.Fatalf("expected 0 products, got %d", len(products))
	}
}

func TestJSONFileStore_SaveAndMerge(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "catalog.json")
	store := NewJSONFileStore(filePath)

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// 1. Initial save of 2 items
	batch1 := []product.Product{
		{
			Source:     "pepperfry",
			ExternalID: "P101",
			URL:        "https://pepperfry.com/p101",
			Name:       "Sofa Initial",
			PriceMinor: 2000000,
			Currency:   "INR",
			Category:   "sofas",
			ScrapedAt:  now,
		},
		{
			Source:     "nestasia",
			ExternalID: "N201",
			URL:        "https://nestasia.in/n201",
			Name:       "Vase Blue",
			PriceMinor: 150000,
			Currency:   "INR",
			Category:   "vases",
			Styles:     []string{"minimalist"},
			ScrapedAt:  now,
		},
	}

	if err := store.Save(ctx, batch1); err != nil {
		t.Fatalf("failed to save batch1: %v", err)
	}

	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load after batch1: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 items, got %d", len(loaded))
	}

	// 2. Save batch2: update P101, add N202, omit N201 (should retain N201)
	batch2 := []product.Product{
		{
			Source:     "pepperfry",
			ExternalID: "P101",
			URL:        "https://pepperfry.com/p101",
			Name:       "Sofa Updated Price",
			PriceMinor: 1800000, // Price updated
			Currency:   "INR",
			Category:   "sofas",
			ScrapedAt:  now.Add(time.Hour),
		},
		{
			Source:     "nestasia",
			ExternalID: "N202",
			URL:        "https://nestasia.in/n202",
			Name:       "Planter Green",
			PriceMinor: 89900,
			Currency:   "INR",
			Category:   "planters",
			Styles:     []string{"bohemian"},
			ScrapedAt:  now.Add(time.Hour),
		},
	}

	if err := store.Save(ctx, batch2); err != nil {
		t.Fatalf("failed to save batch2: %v", err)
	}

	loaded2, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load after batch2: %v", err)
	}

	// Total items should now be 3 (P101 updated, N201 retained, N202 added)
	if len(loaded2) != 3 {
		t.Fatalf("expected 3 merged items, got %d", len(loaded2))
	}

	// Verify items and order (alphabetical by StableKey: nestasia:N201, nestasia:N202, pepperfry:P101)
	if loaded2[0].StableKey() != "nestasia:N201" {
		t.Errorf("expected loaded2[0] to be nestasia:N201, got %s", loaded2[0].StableKey())
	}
	if loaded2[1].StableKey() != "nestasia:N202" {
		t.Errorf("expected loaded2[1] to be nestasia:N202, got %s", loaded2[1].StableKey())
	}
	if loaded2[2].StableKey() != "pepperfry:P101" {
		t.Errorf("expected loaded2[2] to be pepperfry:P101, got %s", loaded2[2].StableKey())
	}
	if loaded2[2].Name != "Sofa Updated Price" || loaded2[2].PriceMinor != 1800000 {
		t.Errorf("expected P101 to be updated with new price, got name=%q, price=%d", loaded2[2].Name, loaded2[2].PriceMinor)
	}

	// Verify no temporary files were left behind in the directory
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("reading tempDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "catalog.json" {
		t.Errorf("expected only catalog.json in directory, found: %v", entries)
	}
}
