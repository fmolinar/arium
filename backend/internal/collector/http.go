package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// UserAgent identifies the collector to upstreams.
const UserAgent = "arium-collector/0.1 (+https://github.com/fmolinar/arium)"

// maxBodyBytes caps how much of a response is read, so a misbehaving
// upstream can't exhaust memory.
const maxBodyBytes = 10 << 20

// HTTPGet fetches url and returns the body, failing on non-2xx responses and
// bodies larger than 10 MiB.
func HTTPGet(ctx context.Context, client *http.Client, url, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", UserAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("get %s: unexpected status %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("get %s: response larger than %d bytes", url, maxBodyBytes)
	}

	return body, nil
}
