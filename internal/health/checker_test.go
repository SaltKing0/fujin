package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckAll_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := New([]string{srv.URL}, 10*time.Second, nil)
	results := c.CheckAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if r.StatusCode != 200 {
		t.Errorf("expected 200, got %d", r.StatusCode)
	}
	if r.LatencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", r.LatencyMs)
	}
}

func TestCheckAll_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New([]string{srv.URL}, 10*time.Second, nil)
	results := c.CheckAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if r.StatusCode != 500 {
		t.Errorf("expected 500, got %d", r.StatusCode)
	}
}

func TestCheckAll_MultipleEndpoints(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv1.Close()
	defer srv2.Close()

	c := New([]string{srv1.URL, srv2.URL}, 10*time.Second, nil)
	results := c.CheckAll()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].StatusCode != 200 {
		t.Errorf("expected 200, got %d", results[0].StatusCode)
	}
	if results[1].StatusCode != 503 {
		t.Errorf("expected 503, got %d", results[1].StatusCode)
	}
}

func TestCheckAll_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New([]string{srv.URL}, 10*time.Second, nil)
	c.Timeout = 100 * time.Millisecond
	c.Client.Timeout = 100 * time.Millisecond

	results := c.CheckAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
