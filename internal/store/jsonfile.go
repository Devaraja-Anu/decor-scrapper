package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"devaraja-anu/decor-scrapper/internal/product"
)

// JSONFileStore persists and merges products into a single formatted JSON file.
type JSONFileStore struct {
	mu       sync.Mutex
	filePath string
}

// NewJSONFileStore creates a new JSONFileStore pointing to the target file path.
func NewJSONFileStore(filePath string) *JSONFileStore {
	return &JSONFileStore{
		filePath: filePath,
	}
}

// Load retrieves all products currently saved in the JSON file.
// If the file does not exist, an empty slice and nil error are returned.
func (s *JSONFileStore) Load(ctx context.Context) ([]product.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadUnlocked()
}

func (s *JSONFileStore) loadUnlocked() ([]product.Product, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []product.Product{}, nil
		}
		return nil, fmt.Errorf("reading catalog file %q: %w", s.filePath, err)
	}

	if len(data) == 0 {
		return []product.Product{}, nil
	}

	var products []product.Product
	if err := json.Unmarshal(data, &products); err != nil {
		return nil, fmt.Errorf("unmarshaling catalog JSON from %q: %w", s.filePath, err)
	}

	return products, nil
}

// Save merges new products into the existing catalog by StableKey and writes atomically.
func (s *JSONFileStore) Save(ctx context.Context, newProducts []product.Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load existing products if any
	existing, err := s.loadUnlocked()
	if err != nil {
		return fmt.Errorf("loading existing catalog before save: %w", err)
	}

	// Merge products keyed by StableKey (source:external_id)
	mergedMap := make(map[string]product.Product, len(existing)+len(newProducts))
	for _, p := range existing {
		mergedMap[p.StableKey()] = p
	}
	for _, p := range newProducts {
		mergedMap[p.StableKey()] = p
	}

	// Convert back to slice and sort deterministically by StableKey for consistent diffs
	mergedList := make([]product.Product, 0, len(mergedMap))
	for _, p := range mergedMap {
		mergedList = append(mergedList, p)
	}
	sort.Slice(mergedList, func(i, j int) bool {
		return mergedList[i].StableKey() < mergedList[j].StableKey()
	})

	// Format as indented JSON
	encoded, err := json.MarshalIndent(mergedList, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling catalog JSON: %w", err)
	}
	// Add trailing newline
	encoded = append(encoded, '\n')

	// Ensure target directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %q: %w", dir, err)
	}

	// Write to temporary file in the same directory for atomic rename
	tempFile, err := os.CreateTemp(dir, "catalog-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for catalog write: %w", err)
	}
	tempPath := tempFile.Name()

	// Clean up temp file on failure
	var writeSuccess bool
	defer func() {
		if !writeSuccess {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(encoded); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("writing to temp file %q: %w", tempPath, err)
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("syncing temp file %q: %w", tempPath, err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temp file %q: %w", tempPath, err)
	}

	// Atomic rename over target file
	if err := os.Rename(tempPath, s.filePath); err != nil {
		return fmt.Errorf("renaming temp file %q to target %q: %w", tempPath, s.filePath, err)
	}

	writeSuccess = true
	return nil
}
