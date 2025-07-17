package jsref

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// httpResolver resolves HTTP/HTTPS references
type httpResolver struct{}

// NewHTTPResolver creates a new HTTP resolver.
// This resolver expects string resources containing HTTP or HTTPS URLs.
// Use this resolver when your resource parameter in Resolve() will be URL strings
// like "https://example.com/data.json" that need to be fetched over HTTP.
func NewHTTPResolver() Resolver {
	return &httpResolver{}
}

// CanResolve returns true if the resource is an HTTP/HTTPS URL string
func (r *httpResolver) CanResolve(resource any) bool {
	str, ok := resource.(string)
	if !ok {
		return false
	}
	u, err := url.Parse(str)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// Resolve resolves HTTP/HTTPS references
func (r *httpResolver) Resolve(dst any, resource any, localRef string) error {
	str, ok := resource.(string)
	if !ok {
		return fmt.Errorf("jsref: httpResolver.Resolve: httpResolver requires string resource, got %T", resource)
	}
	u, err := url.Parse(str)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("jsref: httpResolver.Resolve: httpResolver requires HTTP/HTTPS URL, got: %s", str)
	}

	uri := str

	// Fetch the resource
	data, err := r.fetchHTTP(uri)
	if err != nil {
		return fmt.Errorf("jsref: httpResolver.Resolve: failed to fetch HTTP resource %s: %w", uri, err)
	}

	// Parse the data
	parsed, err := parseData(data)
	if err != nil {
		return err
	}

	// Create an object resolver for the fetched data
	objectResolver := objectResolver{}

	// Resolve against the fetched data
	return objectResolver.Resolve(dst, parsed, localRef)
}

// fetchHTTP fetches content via HTTP
func (r *httpResolver) fetchHTTP(uri string) ([]byte, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("jsref: httpResolver.fetchHTTP: failed to fetch %s: %w", uri, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jsref: httpResolver.fetchHTTP: HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jsref: httpResolver.fetchHTTP: failed to read response body: %w", err)
	}

	return data, nil
}
