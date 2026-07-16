package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	values := []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond, 4 * time.Millisecond, 5 * time.Millisecond}
	if got := percentile(values, 0.95); got != 4*time.Millisecond {
		t.Fatalf("expected nearest-rank index duration, got %s", got)
	}
}

func TestLoadBodiesSortsFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "02.json"), []byte(`{"id":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "01.json"), []byte(`{"id":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bodies, err := loadBodies("", directory, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || string(bodies[0]) != `{"id":1}` || string(bodies[1]) != `{"id":2}` {
		t.Fatalf("unexpected body order: %q", bodies)
	}
}

func TestExecuteBatchReusesHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	results := executeBatch(server.Client(), http.MethodGet, server.URL, "", time.Second, make([][]byte, 8), 4)
	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d", len(results))
	}
	for _, result := range results {
		if result.err != nil || result.status != http.StatusOK {
			t.Fatalf("unexpected result: status=%d error=%v", result.status, result.err)
		}
	}
}
