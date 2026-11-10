package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	baseURL     = flag.String("url", "http://url-shortener-svc:80", "Internal service URL")
	requests    = flag.Int("requests", 1000, "Total number of requests")
	concurrency = flag.Int("concurrency", 10, "Number of concurrent workers")
)

type createResponse struct {
	ShortCode string `json:"short_code"`
}

var (
	successCount int64
	errorCount   int64
	totalLatency int64 // nanoseconds
)

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Create a test short URL first
	shortCode := createShortURL(ctx, client)
	if shortCode == "" {
		fmt.Println("Failed to create short URL")
		os.Exit(1)
	}
	fmt.Printf("Using short code: %s\n", shortCode)

	var wg sync.WaitGroup
	requestsPerWorker := *requests / *concurrency
	remainder := *requests % *concurrency

	start := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		workerReqs := requestsPerWorker
		if i < remainder {
			workerReqs++
		}
		go worker(ctx, i, workerReqs, shortCode, client, &wg)
	}

	wg.Wait()
	elapsed := time.Since(start)

	printStats(elapsed)
}

func createShortURL(ctx context.Context, client *http.Client) string {
	body := bytes.NewBufferString(`{"long_url":"https://go.dev"}`)
	req, err := http.NewRequestWithContext(ctx, "POST", *baseURL+"/api/v1/urls", body)
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result createResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	return result.ShortCode
}

func worker(ctx context.Context, id, numReqs int, shortCode string, client *http.Client, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < numReqs; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Mix: 70% redirect, 20% create, 10% stats
		r := rand.Float64()
		var req *http.Request
		var err error

		switch {
		case r < 0.7:
			req, err = http.NewRequestWithContext(ctx, "GET", *baseURL+"/"+shortCode, nil)
		case r < 0.9:
			body := bytes.NewBufferString(fmt.Sprintf(`{"long_url":"https://example.com/%d"}`, rand.Intn(1000000)))
			req, err = http.NewRequestWithContext(ctx, "POST", *baseURL+"/api/v1/urls", body)
			req.Header.Set("Content-Type", "application/json")
		default:
			req, err = http.NewRequestWithContext(ctx, "GET", *baseURL+"/api/v1/urls/"+shortCode+"/stats", nil)
		}

		if err != nil {
			atomic.AddInt64(&errorCount, 1)
			continue
		}

		reqStart := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(reqStart)
		atomic.AddInt64(&totalLatency, int64(latency))

		if err != nil {
			atomic.AddInt64(&errorCount, 1)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			atomic.AddInt64(&successCount, 1)
		} else {
			atomic.AddInt64(&errorCount, 1)
		}
	}
}

func printStats(elapsed time.Duration) {
	success := atomic.LoadInt64(&successCount)
	errors := atomic.LoadInt64(&errorCount)
	total := success + errors
	avgLatency := time.Duration(0)
	if total > 0 {
		avgLatency = time.Duration(atomic.LoadInt64(&totalLatency) / total)
	}

	fmt.Printf("\n=== Internal Load Test Results ===\n")
	fmt.Printf("Duration:     %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Total Requests: %d\n", total)
	fmt.Printf("Successful:     %d (%.2f%%)\n", success, float64(success)/float64(total)*100)
	fmt.Printf("Errors:         %d (%.2f%%)\n", errors, float64(errors)/float64(total)*100)
	fmt.Printf("RPS:            %.2f\n", float64(total)/elapsed.Seconds())
	fmt.Printf("Avg Latency:    %v\n", avgLatency.Round(time.Microsecond))
}