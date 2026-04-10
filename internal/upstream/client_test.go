package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchCoalescesConcurrentRequests(t *testing.T) {
	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(75 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})

	fetch := func() {
		defer wg.Done()
		<-start

		statusCode, _, body, err := client.Fetch(context.Background(), server.URL)
		if err != nil {
			results <- err
			return
		}
		if statusCode != http.StatusOK {
			results <- &statusError{got: statusCode, want: http.StatusOK}
			return
		}
		if string(body) != "ok" {
			results <- &bodyError{got: string(body), want: "ok"}
			return
		}

		results <- nil
	}

	wg.Add(2)
	go fetch()
	go fetch()
	close(start)
	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := hits.Load(); got != 1 {
		t.Fatalf("expected exactly 1 upstream request, got %d", got)
	}
}

type statusError struct {
	got  int
	want int
}

func (e *statusError) Error() string {
	return "unexpected status code"
}

type bodyError struct {
	got  string
	want string
}

func (e *bodyError) Error() string {
	return "unexpected response body"
}
