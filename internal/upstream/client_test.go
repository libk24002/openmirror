package upstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchRequestForwardsMethodAndHeaders(t *testing.T) {
	methodSeen := ""
	acceptSeen := ""
	authorizationSeen := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodSeen = r.Method
		acceptSeen = r.Header.Get("Accept")
		authorizationSeen = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)

	headers := make(http.Header)
	headers.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	headers.Set("Authorization", "Bearer api-token")

	statusCode, _, body, err := client.FetchRequest(context.Background(), Request{
		Method:  http.MethodHead,
		URL:     server.URL + "?from=test",
		Headers: headers,
	})
	if err != nil {
		t.Fatalf("fetch request returned error: %v", err)
	}
	if statusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusNoContent)
	}
	if len(body) != 0 {
		t.Fatalf("body len = %d, want %d", len(body), 0)
	}
	if methodSeen != http.MethodHead {
		t.Fatalf("upstream method = %q, want %q", methodSeen, http.MethodHead)
	}
	if acceptSeen != "application/vnd.oci.image.manifest.v1+json" {
		t.Fatalf("upstream accept = %q", acceptSeen)
	}
	if authorizationSeen != "Bearer api-token" {
		t.Fatalf("upstream authorization = %q", authorizationSeen)
	}
}

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

func TestFetchLeaderCancellationDoesNotCancelFollower(t *testing.T) {
	var hits atomic.Int32
	started := make(chan struct{})
	allowResponse := make(chan struct{})
	var allowResponseOnce sync.Once
	closeAllowResponse := func() {
		allowResponseOnce.Do(func() {
			close(allowResponse)
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-started:
		default:
			close(started)
		}

		<-allowResponse
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	defer closeAllowResponse()

	client := NewClient(2 * time.Second)

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()

	type fetchOutcome struct {
		status int
		body   []byte
		err    error
	}

	leaderDone := make(chan fetchOutcome, 1)
	followerDone := make(chan fetchOutcome, 1)

	go func() {
		statusCode, _, body, err := client.Fetch(leaderCtx, server.URL)
		leaderDone <- fetchOutcome{status: statusCode, body: body, err: err}
	}()

	<-started

	go func() {
		statusCode, _, body, err := client.Fetch(context.Background(), server.URL)
		followerDone <- fetchOutcome{status: statusCode, body: body, err: err}
	}()

	time.Sleep(25 * time.Millisecond)
	cancelLeader()

	leaderResult := <-leaderDone
	if !errors.Is(leaderResult.err, context.Canceled) {
		t.Fatalf("expected leader to return context canceled, got %v", leaderResult.err)
	}

	closeAllowResponse()

	followerResult := <-followerDone
	if followerResult.err != nil {
		t.Fatalf("expected follower to succeed, got %v", followerResult.err)
	}
	if followerResult.status != http.StatusOK {
		t.Fatalf("expected follower status %d, got %d", http.StatusOK, followerResult.status)
	}
	if string(followerResult.body) != "ok" {
		t.Fatalf("expected follower body %q, got %q", "ok", string(followerResult.body))
	}

	if got := hits.Load(); got != 1 {
		t.Fatalf("expected exactly 1 upstream request, got %d", got)
	}
}

func TestFetchFollowerCancellationReturnsQuickly(t *testing.T) {
	var hits atomic.Int32
	started := make(chan struct{})
	allowResponse := make(chan struct{})
	var allowResponseOnce sync.Once
	closeAllowResponse := func() {
		allowResponseOnce.Do(func() {
			close(allowResponse)
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-started:
		default:
			close(started)
		}

		<-allowResponse
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	defer closeAllowResponse()

	client := NewClient(2 * time.Second)

	type fetchOutcome struct {
		status int
		body   []byte
		err    error
	}

	leaderDone := make(chan fetchOutcome, 1)
	followerDone := make(chan fetchOutcome, 1)

	go func() {
		statusCode, _, body, err := client.Fetch(context.Background(), server.URL)
		leaderDone <- fetchOutcome{status: statusCode, body: body, err: err}
	}()

	<-started

	followerCtx, cancelFollower := context.WithCancel(context.Background())
	defer cancelFollower()

	go func() {
		statusCode, _, body, err := client.Fetch(followerCtx, server.URL)
		followerDone <- fetchOutcome{status: statusCode, body: body, err: err}
	}()

	time.Sleep(25 * time.Millisecond)
	cancelFollower()

	start := time.Now()
	select {
	case followerResult := <-followerDone:
		if !errors.Is(followerResult.err, context.Canceled) {
			t.Fatalf("expected follower cancellation error, got %v", followerResult.err)
		}
		if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
			t.Fatalf("expected follower cancellation to return quickly, took %s", elapsed)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected follower cancellation to return before upstream response")
	}

	select {
	case <-leaderDone:
		t.Fatal("expected leader fetch to still be waiting for upstream response")
	default:
	}

	closeAllowResponse()

	leaderResult := <-leaderDone
	if leaderResult.err != nil {
		t.Fatalf("expected leader to succeed, got %v", leaderResult.err)
	}
	if leaderResult.status != http.StatusOK {
		t.Fatalf("expected leader status %d, got %d", http.StatusOK, leaderResult.status)
	}
	if string(leaderResult.body) != "ok" {
		t.Fatalf("expected leader body %q, got %q", "ok", string(leaderResult.body))
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
