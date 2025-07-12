package jsref

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lestrrat-go/jsptr"
	"github.com/goccy/go-yaml"
)

// Resolver is the main interface for resolving JSON references
type Resolver interface {
	Resolve(dst any, reference string) error
}

// StackedResolver combines multiple resolvers and tries them in order
type StackedResolver struct {
	resolvers []Resolver
}

// AddResolver adds a resolver to the stack
func (r *StackedResolver) AddResolver(resolver Resolver) {
	r.resolvers = append(r.resolvers, resolver)
}

// Resolve tries each resolver in order until one succeeds
func (r *StackedResolver) Resolve(dst any, reference string) error {
	var lastErr error
	for _, resolver := range r.resolvers {
		err := resolver.Resolve(dst, reference)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return fmt.Errorf("all resolvers failed, last error: %w", lastErr)
	}
	return fmt.Errorf("no resolvers available")
}

// localResolver resolves pointers against a single static object
type localResolver struct {
	object any
}

// New creates a new StackedResolver with a local resolver for the given object
func New(object any) *StackedResolver {
	stacked := &StackedResolver{resolvers: make([]Resolver, 0)}
	stacked.AddResolver(&localResolver{object: object})
	return stacked
}

// NewLocalResolver creates a new localResolver for the given object
func NewLocalResolver(object any) Resolver {
	return &localResolver{object: object}
}

// Resolve resolves a reference against the local object
func (r *localResolver) Resolve(dst any, reference string) error {
	// Local references must start with "#"
	if !strings.HasPrefix(reference, "#") {
		return fmt.Errorf("local references must start with '#', got: %s", reference)
	}
	
	// Remove the "#" prefix to get the JSON pointer
	pointer := reference[1:]
	ptr, err := jsptr.New(pointer)
	if err != nil {
		return fmt.Errorf("invalid JSON pointer %s: %w", pointer, err)
	}
	return ptr.Retrieve(dst, r.object)
}

// dynamicResolver fetches external resources dynamically
type dynamicResolver struct{}

// NewDynamicResolver creates a new dynamicResolver
func NewDynamicResolver() Resolver {
	return &dynamicResolver{}
}

// Resolve resolves a reference string that may reference external resources
func (r *dynamicResolver) Resolve(dst any, reference string) error {
	// Check if this is a local reference only (starts with #)
	if strings.HasPrefix(reference, "#") {
		// Dynamic resolver cannot handle local-only references
		return fmt.Errorf("dynamic resolver cannot handle local reference: %s", reference)
	}

	// Parse the reference to separate URI and fragment
	uri, fragment := parseReference(reference)
	
	// Fetch the resource
	data, err := r.fetchResource(uri)
	if err != nil {
		return fmt.Errorf("failed to fetch resource %s: %w", uri, err)
	}

	// Create a local resolver for the fetched data
	localResolver := NewLocalResolver(data)
	
	// External references must have a fragment (starting with #)
	if fragment == "" {
		// No fragment means return the whole document
		return localResolver.Resolve(dst, "#") // Empty pointer means root
	}
	
	// Resolve with the fragment (which should start with #)
	return localResolver.Resolve(dst, "#"+fragment)
}

// fetchResource fetches content from various sources
func (r *dynamicResolver) fetchResource(uri string) (any, error) {
	// Parse the URI
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI %s: %w", uri, err)
	}

	var data []byte
	
	// Handle different schemes
	switch u.Scheme {
	case "http", "https":
		data, err = r.fetchHTTP(uri)
	case "file", "":
		// Handle file:// URLs and relative paths
		path := u.Path
		if u.Scheme == "" {
			path = uri
		}
		data, err = r.fetchFile(path)
	default:
		return nil, fmt.Errorf("unsupported URI scheme: %s", u.Scheme)
	}
	
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON first, then YAML
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		// Try YAML if JSON fails
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse as JSON or YAML: %w", err)
		}
	}
	
	return parsed, nil
}

// fetchHTTP fetches content via HTTP
func (r *dynamicResolver) fetchHTTP(uri string) ([]byte, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", uri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// fetchFile fetches content from local files
func (r *dynamicResolver) fetchFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return data, nil
}

// fileResolver resolves references to static files
type fileResolver struct {
	path string
}

// NewFileResolver creates a new fileResolver for the given file path
func NewFileResolver(path string) Resolver {
	return &fileResolver{path: path}
}

// Resolve resolves a reference against the static file
func (r *fileResolver) Resolve(dst any, reference string) error {
	// For file resolver, the reference should be a local reference starting with #
	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", r.path, err)
	}

	// Parse the data based on file extension
	var parsed any
	ext := strings.ToLower(filepath.Ext(r.path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("failed to parse JSON file %s: %w", r.path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("failed to parse YAML file %s: %w", r.path, err)
		}
	default:
		// Assume JSON if no extension or unknown extension
		if err := json.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("failed to parse file %s as JSON: %w", r.path, err)
		}
	}

	localResolver := NewLocalResolver(parsed)
	return localResolver.Resolve(dst, reference)
}

// uriResolver resolves references to static URIs
type uriResolver struct {
	uri string
}

// NewURIResolver creates a new uriResolver for the given URI
func NewURIResolver(uri string) Resolver {
	return &uriResolver{uri: uri}
}

// Resolve resolves a reference against the static URI
func (r *uriResolver) Resolve(dst any, reference string) error {
	// For URI resolver, the reference should be a local reference starting with #
	dynamicRes := &dynamicResolver{}
	data, err := dynamicRes.fetchResource(r.uri)
	if err != nil {
		return fmt.Errorf("failed to fetch URI %s: %w", r.uri, err)
	}

	localResolver := NewLocalResolver(data)
	return localResolver.Resolve(dst, reference)
}



// parseReference separates URI and fragment parts
func parseReference(ref string) (uri, fragment string) {
	if idx := strings.Index(ref, "#"); idx >= 0 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}