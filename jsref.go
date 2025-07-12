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

	"github.com/goccy/go-yaml"
	"github.com/lestrrat-go/jsptr"
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

// objectResolver resolves pointers against a single static object
type objectResolver struct {
	object any
}

// New creates a new StackedResolver with an object resolver for the given object
func New(object any) *StackedResolver {
	stacked := &StackedResolver{resolvers: make([]Resolver, 0)}
	stacked.AddResolver(&objectResolver{object: object})
	return stacked
}

// NewObjectResolver creates a new objectResolver for the given object
func NewObjectResolver(object any) Resolver {
	return &objectResolver{object: object}
}

// Resolve resolves a reference against the object
func (r *objectResolver) Resolve(dst any, reference string) error {
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

// httpResolver resolves HTTP/HTTPS references
type httpResolver struct{}

// NewHTTPResolver creates a new httpResolver
func NewHTTPResolver() Resolver {
	return &httpResolver{}
}

// Resolve resolves HTTP/HTTPS references that may include fragments
func (r *httpResolver) Resolve(dst any, reference string) error {
	// Check if this is a local reference only (starts with #)
	if strings.HasPrefix(reference, "#") {
		return fmt.Errorf("HTTP resolver cannot handle local reference: %s", reference)
	}

	// Parse the reference to separate URI and fragment
	uri, fragment := parseReference(reference)

	// Verify this is an HTTP/HTTPS URL
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("invalid URI %s: %w", uri, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("HTTP resolver can only handle http/https schemes, got: %s", u.Scheme)
	}

	// Fetch the resource
	data, err := r.fetchHTTP(uri)
	if err != nil {
		return fmt.Errorf("failed to fetch HTTP resource %s: %w", uri, err)
	}

	// Parse the data
	parsed, err := r.parseData(data)
	if err != nil {
		return err
	}

	// Create an object resolver for the fetched data
	objectResolver := NewObjectResolver(parsed)

	// External references must have a fragment (starting with #)
	if fragment == "" {
		// No fragment means return the whole document
		return objectResolver.Resolve(dst, "#") // Empty pointer means root
	}

	// Resolve with the fragment (which should start with #)
	return objectResolver.Resolve(dst, "#"+fragment)
}

// fetchHTTP fetches content via HTTP
func (r *httpResolver) fetchHTTP(uri string) ([]byte, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", uri, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// parseData parses JSON or YAML data
func (r *httpResolver) parseData(data []byte) (any, error) {
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

// fsResolver resolves file-based references using os.Root
type fsResolver struct {
	root    *os.Root
	rootDir string
}

// NewFSResolver creates a new fsResolver with the given directory
// If dir is empty, it defaults to the current directory
func NewFSResolver(dir string) (Resolver, error) {
	if dir == "" {
		dir = "."
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory %s: %w", dir, err)
	}
	return &fsResolver{root: root, rootDir: dir}, nil
}

// Resolve resolves file references that may include fragments
func (r *fsResolver) Resolve(dst any, reference string) error {
	// Check if this is a local reference only (starts with #)
	if strings.HasPrefix(reference, "#") {
		return fmt.Errorf("fs resolver cannot handle local reference: %s", reference)
	}

	// Parse the reference to separate file path and fragment
	filePath, fragment := parseReference(reference)

	// Handle file:// URLs by extracting the path
	if u, err := url.Parse(filePath); err == nil {
		if u.Scheme == "http" || u.Scheme == "https" {
			return fmt.Errorf("fs resolver cannot handle HTTP/HTTPS URLs: %s", filePath)
		}
		if u.Scheme == "file" {
			filePath = u.Path
		}
	}

	// Handle absolute paths by checking if they're within our root
	if filepath.IsAbs(filePath) {
		// Make rootDir absolute for comparison
		rootDir := r.rootDir
		if !filepath.IsAbs(rootDir) {
			var err error
			rootDir, err = filepath.Abs(rootDir)
			if err != nil {
				return fmt.Errorf("failed to resolve root directory: %w", err)
			}
		}

		// Check if the absolute path is within our root
		relPath, err := filepath.Rel(rootDir, filePath)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return fmt.Errorf("fs resolver cannot handle path outside its root: %s (root: %s)", filePath, rootDir)
		}
		filePath = relPath
	}

	// Clean the file path
	filePath = filepath.Clean(filePath)

	// Check if root was successfully opened
	if r.root == nil {
		return fmt.Errorf("fs resolver root directory is not accessible: %s", r.rootDir)
	}

	// Read the file using os.Root
	file, err := r.root.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Parse the data based on file extension
	var parsed any
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("failed to parse JSON file %s: %w", filePath, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("failed to parse YAML file %s: %w", filePath, err)
		}
	default:
		// Assume JSON if no extension or unknown extension
		if err := json.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("failed to parse file %s as JSON: %w", filePath, err)
		}
	}

	// Create an object resolver for the loaded data
	objectResolver := NewObjectResolver(parsed)

	// External references must have a fragment (starting with #)
	if fragment == "" {
		// No fragment means return the whole document
		return objectResolver.Resolve(dst, "#") // Empty pointer means root
	}

	// Resolve with the fragment (which should start with #)
	return objectResolver.Resolve(dst, "#"+fragment)
}

// parseReference separates URI and fragment parts
func parseReference(ref string) (uri, fragment string) {
	if idx := strings.Index(ref, "#"); idx >= 0 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}
