package lookup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ErrUnavailable is a source that refused the request outright (a key it will not take,
// an address it will not serve). Asking again will not help, so the merge goes on without it.
var ErrUnavailable = errors.New("source unavailable")

const maxBody = 8 << 20

// fetcher is one source's HTTP: a polite gap between requests, the app's name in every
// request, and one retry when the connection drops, which the Library of Congress does
// now and then under load.
type fetcher struct {
	client    *http.Client
	userAgent string
	gap       time.Duration

	mu   sync.Mutex
	last time.Time
}

func newFetcher(client *http.Client, userAgent string, gap time.Duration) *fetcher {
	return &fetcher{client: client, userAgent: userAgent, gap: gap}
}

func (f *fetcher) wait(ctx context.Context) error {
	f.mu.Lock()
	next := f.last.Add(f.gap)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	f.last = next
	f.mu.Unlock()
	t := time.NewTimer(time.Until(next))
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// get returns the body of a 200, nil for a 404, ErrUnavailable for a 401 or 403, and an
// error for anything else.
func (f *fetcher) get(ctx context.Context, url string) ([]byte, error) {
	return f.do(ctx, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	})
}

// post sends a JSON body with an authorization, as a GraphQL API takes a query, and answers
// as get does.
func (f *fetcher) post(ctx context.Context, url string, body []byte, authorization string) ([]byte, error) {
	return f.do(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", authorization)
		return req, nil
	})
}

// do sends the request build makes, once more if the connection drops.
func (f *fetcher) do(ctx context.Context, build func() (*http.Request, error)) ([]byte, error) {
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if err := f.wait(ctx); err != nil {
			return nil, err
		}
		body, retry, err := f.once(ctx, build)
		if err == nil || !retry {
			return body, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func (f *fetcher) once(ctx context.Context, build func() (*http.Request, error)) (body []byte, retry bool, err error) {
	req, err := build()
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", f.userAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		// The error names the address, and Google's carries the key in its query: it goes, so
		// no log, stored error, or answer shows it.
		var ue *url.Error
		if errors.As(err, &ue) {
			if u, perr := url.Parse(ue.URL); perr == nil {
				u.RawQuery = ""
				ue.URL = u.String()
			}
		}
		return nil, ctx.Err() == nil, err
	}
	defer func() { _ = resp.Body.Close() }() // read to the end below; nothing left to report
	switch {
	case resp.StatusCode == http.StatusOK:
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		return b, err != nil, err
	case resp.StatusCode == http.StatusNotFound:
		return nil, false, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, false, fmt.Errorf("%w: %s", ErrUnavailable, resp.Status)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("%s", resp.Status)
	}
	return nil, false, fmt.Errorf("%s", resp.Status)
}
