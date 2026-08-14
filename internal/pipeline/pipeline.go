package pipeline

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/time/rate"

	"devaraja-anu/decor-scrapper/internal/product"
	"devaraja-anu/decor-scrapper/internal/site"
	"devaraja-anu/decor-scrapper/internal/store"
)

// Default concurrency and rate limiting values.
const (
	DefaultWorkers   = 4
	DefaultRateLimit = rate.Limit(5) // 5 requests per second per site
	DefaultBurst     = 2
)

// Config configures the pipeline execution.
type Config struct {
	Workers      int
	RateLimit    rate.Limit
	Burst        int
	OnFetchError func(siteName string, ref product.ProductRef, err error)
}

// Pipeline orchestrates concurrent scraping across sites and writes to the store.
type Pipeline struct {
	store    store.Store
	sites    map[string]site.Site
	limiters map[string]*rate.Limiter
	cfg      Config
	mu       sync.Mutex
}

type workItem struct {
	siteName string
	ref      product.ProductRef
}

// New creates a new configured Pipeline.
func New(st store.Store, sites []site.Site, cfg Config) *Pipeline {
	if cfg.Workers <= 0 {
		cfg.Workers = DefaultWorkers
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = DefaultRateLimit
	}
	if cfg.Burst <= 0 {
		cfg.Burst = DefaultBurst
	}

	siteMap := make(map[string]site.Site, len(sites))
	limiters := make(map[string]*rate.Limiter, len(sites))
	for _, s := range sites {
		siteMap[s.Name()] = s
		limiters[s.Name()] = rate.NewLimiter(cfg.RateLimit, cfg.Burst)
	}

	return &Pipeline{
		store:    st,
		sites:    siteMap,
		limiters: limiters,
		cfg:      cfg,
	}
}

// Run executes category discovery, parallel product ingestion, and storage.
func (p *Pipeline) Run(ctx context.Context) ([]product.Product, error) {
	// Phase 1: Discovery across all sites and categories
	var allItems []workItem

	for siteName, s := range p.sites {
		for _, cat := range s.Categories() {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if err := p.waitRateLimit(ctx, siteName); err != nil {
				return nil, err
			}

			refs, err := s.ListProducts(ctx, cat)
			if err != nil {
				if p.cfg.OnFetchError != nil {
					p.cfg.OnFetchError(siteName, product.ProductRef{Category: cat.Name}, err)
				}
				continue
			}

			for _, ref := range refs {
				allItems = append(allItems, workItem{
					siteName: siteName,
					ref:      ref,
				})
			}
		}
	}

	if len(allItems) == 0 {
		return []product.Product{}, nil
	}

	// Phase 2: Parallel Fetching via Worker Pool
	jobs := make(chan workItem, len(allItems))
	results := make(chan product.Product, len(allItems))

	for _, item := range allItems {
		jobs <- item
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < p.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if err := p.waitRateLimit(ctx, item.siteName); err != nil {
					return
				}

				s := p.sites[item.siteName]
				prod, err := s.FetchProduct(ctx, item.ref)
				if err != nil {
					if p.cfg.OnFetchError != nil {
						p.cfg.OnFetchError(item.siteName, item.ref, err)
					}
					continue
				}

				results <- prod
			}
		}()
	}

	wg.Wait()
	close(results)

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Phase 3: Collection & Storage
	products := make([]product.Product, 0, len(results))
	for prod := range results {
		products = append(products, prod)
	}

	if p.store != nil && len(products) > 0 {
		if err := p.store.Save(ctx, products); err != nil {
			return products, fmt.Errorf("saving products to store: %w", err)
		}
	}

	return products, nil
}

func (p *Pipeline) waitRateLimit(ctx context.Context, siteName string) error {
	p.mu.Lock()
	lim, ok := p.limiters[siteName]
	if !ok {
		lim = rate.NewLimiter(p.cfg.RateLimit, p.cfg.Burst)
		p.limiters[siteName] = lim
	}
	p.mu.Unlock()

	return lim.Wait(ctx)
}
