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
	v, err, _ := c.group.Do(url, func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	if err != nil {
		return 0, nil, nil, err
	}

	result := v.(fetchResult)
	body := append([]byte(nil), result.body...)

	return result.statusCode, result.headers.Clone(), body, nil
}
