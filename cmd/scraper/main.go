package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"devaraja-anu/decor-scrapper/internal/pipeline"
	"devaraja-anu/decor-scrapper/internal/product"
	"devaraja-anu/decor-scrapper/internal/site"
	"devaraja-anu/decor-scrapper/internal/site/nestasia"
	"devaraja-anu/decor-scrapper/internal/site/pepperfry"
	"devaraja-anu/decor-scrapper/internal/store"
)

func main() {
	outPath := flag.String("out", "catalog.json", "Destination path for output catalog JSON file")
	workers := flag.Int("workers", 4, "Number of concurrent fetch workers")
	rateLimit := flag.Float64("rate", 3.0, "Max requests per second per target site")
	siteFilter := flag.String("site", "", "Filter to specific site ('nestasia', 'pepperfry', or empty for all)")
	timeout := flag.Duration("timeout", 10*time.Minute, "Overall scrape execution timeout")
	flag.Parse()

	// Handle graceful termination on Ctrl+C or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	availableSites := []site.Site{
		nestasia.New(),
		pepperfry.New(),
	}

	var activeSites []site.Site
	filter := strings.ToLower(strings.TrimSpace(*siteFilter))

	for _, s := range availableSites {
		if filter == "" || strings.ToLower(s.Name()) == filter {
			activeSites = append(activeSites, s)
		}
	}

	if len(activeSites) == 0 {
		log.Fatalf("error: no matching sites found for filter %q (available: 'nestasia', 'pepperfry')", *siteFilter)
	}

	log.Printf("== Decor Scraper ==")
	log.Printf("Target sites : %s", formatSiteNames(activeSites))
	log.Printf("Output file  : %s", *outPath)
	log.Printf("Concurrency  : %d workers | %.1f req/s per site", *workers, *rateLimit)

	jsonStore := store.NewJSONFileStore(*outPath)

	cfg := pipeline.Config{
		Workers:   *workers,
		RateLimit: rate.Limit(*rateLimit),
		Burst:     2,
		OnFetchError: func(siteName string, ref product.ProductRef, err error) {
			log.Printf("[%s] fetch error (%s): %v", siteName, ref.URL, err)
		},
	}

	pipe := pipeline.New(jsonStore, activeSites, cfg)

	startTime := time.Now()
	log.Printf("Starting discovery and parallel ingestion...")

	products, err := pipe.Run(ctx)
	if err != nil {
		if ctx.Err() != nil {
			log.Printf("Scrape interrupted: %v", ctx.Err())
			os.Exit(130)
		}
		log.Fatalf("Pipeline execution failed: %v", err)
	}

	elapsed := time.Since(startTime).Round(time.Millisecond)
	log.Printf("== Complete ==")
	log.Printf("Scraped %d products in %v", len(products), elapsed)

	// Verify saved catalog size
	saved, err := jsonStore.Load(context.Background())
	if err == nil {
		log.Printf("Total products in %s: %d", *outPath, len(saved))
	}
}

func formatSiteNames(sites []site.Site) string {
	names := make([]string, len(sites))
	for i, s := range sites {
		names[i] = s.Name()
	}
	return strings.Join(names, ", ")
}
