package upstream

import (
	"context"
	"io"
	"net/http"
	"time"

	"golang.org/x/sync/singleflight"
)

type Client struct {
	httpClient *http.Client
	group      singleflight.Group
}

type fetchResult struct {
	statusCode int
	headers    http.Header
	body       []byte
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Fetch(ctx context.Context, url string) (int, http.Header, []byte, error) {
	resultCh := c.group.DoChan(url, func() (interface{}, error) {
		requestCtx := context.Background()
		cancel := func() {}
		if timeout := c.httpClient.Timeout; timeout > 0 {
			requestCtx, cancel = context.WithTimeout(requestCtx, timeout)
		} else {
			requestCtx, cancel = context.WithCancel(requestCtx)
		}
		defer cancel()

		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
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
