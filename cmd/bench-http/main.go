package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type requestResult struct {
	duration time.Duration
	status   int
	err      error
}

func executeRequest(client *http.Client, method string, requestURL string, authorization string, timeout time.Duration, body []byte) requestResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), requestURL, bytes.NewReader(body))
	if err != nil {
		return requestResult{err: err}
	}
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		request.Header.Set("Authorization", "Bearer "+authorization)
	}

	startedAt := time.Now()
	response, err := client.Do(request)
	duration := time.Since(startedAt)
	if err != nil {
		return requestResult{duration: duration, err: err}
	}
	_, copyErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if copyErr != nil {
		err = copyErr
	} else if closeErr != nil {
		err = closeErr
	}
	return requestResult{duration: duration, status: response.StatusCode, err: err}
}

func executeBatch(client *http.Client, method string, requestURL string, authorization string, timeout time.Duration, bodies [][]byte, concurrency int) []requestResult {
	if concurrency > len(bodies) {
		concurrency = len(bodies)
	}
	jobs := make(chan []byte)
	results := make(chan requestResult, len(bodies))
	var workers sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for body := range jobs {
				results <- executeRequest(client, method, requestURL, authorization, timeout, body)
			}
		}()
	}
	go func() {
		for _, body := range bodies {
			jobs <- body
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	batch := make([]requestResult, 0, len(bodies))
	for result := range results {
		batch = append(batch, result)
	}
	return batch
}

func loadBodies(bodyFile string, bodyDir string, requests int) ([][]byte, error) {
	if bodyFile != "" && bodyDir != "" {
		return nil, fmt.Errorf("body-file 和 body-dir 不能同时使用")
	}
	if bodyDir != "" {
		paths, err := filepath.Glob(filepath.Join(bodyDir, "*.json"))
		if err != nil {
			return nil, err
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			return nil, fmt.Errorf("body-dir 中没有 JSON 文件")
		}
		bodies := make([][]byte, 0, len(paths))
		for _, path := range paths {
			body, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			bodies = append(bodies, body)
		}
		return bodies, nil
	}

	body := []byte(nil)
	if bodyFile != "" {
		value, err := os.ReadFile(bodyFile)
		if err != nil {
			return nil, err
		}
		body = value
	}
	bodies := make([][]byte, requests)
	for index := range bodies {
		bodies[index] = body
	}
	return bodies, nil
}

func percentile(sorted []time.Duration, ratio float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(float64(len(sorted)-1) * ratio)
	return sorted[index]
}

func main() {
	url := flag.String("url", "", "request URL")
	method := flag.String("method", http.MethodGet, "HTTP method")
	requestCount := flag.Int("requests", 200, "total request count when body-dir is not used")
	concurrency := flag.Int("concurrency", 50, "worker count")
	authorization := flag.String("authorization", "", "Bearer token; prefer BENCH_HTTP_AUTHORIZATION environment variable")
	bodyFile := flag.String("body-file", "", "JSON body reused by every request")
	bodyDir := flag.String("body-dir", "", "directory containing one JSON request body per file")
	expectedStatus := flag.Int("expect-status", http.StatusOK, "expected HTTP status")
	warmupRequests := flag.Int("warmup-requests", 0, "requests excluded from measurements and used to establish connections")
	p95Limit := flag.Duration("p95-limit", 0, "optional P95 latency limit, for example 500ms")
	timeout := flag.Duration("timeout", 10*time.Second, "per-request timeout")
	flag.Parse()

	if *url == "" || *requestCount <= 0 || *concurrency <= 0 || *warmupRequests < 0 {
		fmt.Fprintln(os.Stderr, "url, requests, and concurrency must be positive")
		os.Exit(2)
	}
	if *authorization == "" {
		*authorization = os.Getenv("BENCH_HTTP_AUTHORIZATION")
	}
	bodies, err := loadBodies(*bodyFile, *bodyDir, *requestCount)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *concurrency > len(bodies) {
		*concurrency = len(bodies)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = *concurrency
	transport.MaxIdleConnsPerHost = *concurrency
	client := &http.Client{Transport: transport}
	if *warmupRequests > 0 {
		warmupBodies := make([][]byte, *warmupRequests)
		for index := range warmupBodies {
			warmupBodies[index] = bodies[index%len(bodies)]
		}
		for _, result := range executeBatch(client, *method, *url, *authorization, *timeout, warmupBodies, *concurrency) {
			if result.err != nil || result.status != *expectedStatus {
				fmt.Fprintf(os.Stderr, "warmup failed: status=%d error=%v\n", result.status, result.err)
				os.Exit(1)
			}
		}
	}

	startedAt := time.Now()
	results := executeBatch(client, *method, *url, *authorization, *timeout, bodies, *concurrency)

	durations := make([]time.Duration, 0, len(bodies))
	statusCounts := make(map[int]int)
	errorCount := 0
	for _, result := range results {
		durations = append(durations, result.duration)
		if result.err != nil {
			errorCount++
			continue
		}
		statusCounts[result.status]++
	}
	elapsed := time.Since(startedAt)
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := percentile(durations, 0.50)
	p95 := percentile(durations, 0.95)
	p99 := percentile(durations, 0.99)
	rps := float64(len(bodies)) / elapsed.Seconds()

	fmt.Printf("requests=%d concurrency=%d elapsed=%s rps=%.2f p50=%s p95=%s p99=%s statuses=%v errors=%d\n",
		len(bodies), *concurrency, elapsed.Round(time.Millisecond), rps, p50.Round(time.Millisecond), p95.Round(time.Millisecond), p99.Round(time.Millisecond), statusCounts, errorCount)
	if errorCount > 0 || statusCounts[*expectedStatus] != len(bodies) {
		os.Exit(1)
	}
	if *p95Limit > 0 && p95 > *p95Limit {
		fmt.Fprintf(os.Stderr, "P95 %s exceeded limit %s\n", p95, *p95Limit)
		os.Exit(1)
	}
}
