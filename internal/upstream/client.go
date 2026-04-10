package upstream

import (
	"context"
	"io"
	"net/http"
	"net/textproto"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

type Client struct {
	httpClient *http.Client
	group      singleflight.Group
}

type Request struct {
	Method  string
	URL     string
	Headers http.Header
}

type fetchResult struct {
	statusCode int
	headers    http.Header
	body       []byte
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Fetch(ctx context.Context, url string) (int, http.Header, []byte, error) {
	return c.FetchRequest(ctx, Request{Method: http.MethodGet, URL: url})
}

func (c *Client) DoRequest(ctx context.Context, request Request) (*http.Response, error) {
	normalized := normalizeRequest(request)

	requestCtx, cancel := c.timeoutContext(ctx)
	resp, err := c.doRequest(requestCtx, normalized)
	if err != nil {
		cancel()
		return nil, err
	}

	resp.Body = &cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func (c *Client) FetchRequest(ctx context.Context, request Request) (int, http.Header, []byte, error) {
	normalized := normalizeRequest(request)
	resultCh := c.group.DoChan(singleflightKey(normalized), func() (interface{}, error) {
		requestCtx, cancel := c.timeoutContext(context.Background())
		defer cancel()

		resp, err := c.doRequest(requestCtx, normalized)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return fetchResult{
			statusCode: resp.StatusCode,
			headers:    resp.Header.Clone(),
			body:       body,
		}, nil
	})

	select {
	case <-ctx.Done():
		return 0, nil, nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return 0, nil, nil, result.Err
		}

		fetch := result.Val.(fetchResult)
		body := append([]byte(nil), fetch.body...)

		return fetch.statusCode, fetch.headers.Clone(), body, nil
	}
}

func (c *Client) timeoutContext(parent context.Context) (context.Context, context.CancelFunc) {
	if timeout := c.httpClient.Timeout; timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}

	return context.WithCancel(parent)
}

func (c *Client) doRequest(ctx context.Context, request Request) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, request.Method, request.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header = request.Headers.Clone()

	return c.httpClient.Do(req)
}

func normalizeRequest(request Request) Request {
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		method = http.MethodGet
	}

	return Request{
		Method:  method,
		URL:     request.URL,
		Headers: request.Headers.Clone(),
	}
}

func singleflightKey(request Request) string {
	normalizedHeaders := make(map[string][]string, len(request.Headers))
	headerNames := make([]string, 0, len(request.Headers))

	for key, values := range request.Headers {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		if _, exists := normalizedHeaders[canonical]; !exists {
			headerNames = append(headerNames, canonical)
		}
		normalizedHeaders[canonical] = append(normalizedHeaders[canonical], values...)
	}

	sort.Strings(headerNames)

	parts := make([]string, 0, 2+len(headerNames))
	parts = append(parts, request.Method, request.URL)
	for _, name := range headerNames {
		values := append([]string(nil), normalizedHeaders[name]...)
		sort.Strings(values)
		parts = append(parts, name+"="+strings.Join(values, ","))
	}

	return strings.Join(parts, "\n")
}
