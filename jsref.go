package jsref

import (
	"encoding/json"
	"errors"
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

// Resolver is the main interface for resolving JSON references.
type Resolver interface {
	// CanResolve returns true if this resolver can handle the given resource type
	CanResolve(resource any) bool

	// Resolve resolves a JSON reference against a resource.
	// The standard behavior is to use the resource parameter as-is and expect
	// localRef to be a local reference starting with "#" (e.g., "#/path/to/data").
	// Implementations may return an error if the resource type is incompatible.
	Resolve(dst any, resource any, localRef string) error
}

// Split splits a JSON reference into its external and local components
func Split(reference string) (external string, local string, err error) {
	if reference == "" {
		return "", "", fmt.Errorf("jsref.Split: empty reference")
	}

	// If it starts with #, it's a pure local reference
	if strings.HasPrefix(reference, "#") {
		return "", reference, nil
	}

	// Find the # separator
	if idx := strings.Index(reference, "#"); idx >= 0 {
		return reference[:idx], reference[idx:], nil
	}

	// No # found, it's a pure external reference
	return reference, "", nil
}

// parseData parses JSON or YAML data using heuristics for efficient format detection
func parseData(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("jsref.parseData: empty data")
	}

	var parsed any

	// Use heuristics to detect likely format
	if isLikelyJSON(data) {
		// Try JSON first if it looks like JSON
		if err := json.Unmarshal(data, &parsed); err == nil {
			return parsed, nil
		}
		// If JSON failed, still try YAML as fallback
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("jsref.parseData: failed to parse as JSON or YAML: %w", err)
		}
	} else {
		// Try YAML first if it doesn't look like JSON
		if err := yaml.Unmarshal(data, &parsed); err == nil {
			return parsed, nil
		}
		// If YAML failed, try JSON as fallback
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("jsref.parseData: failed to parse as JSON or YAML: %w", err)
		}
	}

	return parsed, nil
}

// isLikelyJSON uses simple heuristics to detect if data is likely JSON
func isLikelyJSON(data []byte) bool {
	// Trim whitespace from start and end
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return false
	}

	// JSON objects/arrays start with { or [
	firstChar := trimmed[0]
	lastChar := trimmed[len(trimmed)-1]

	// Strong indicators of JSON
	if (firstChar == '{' && lastChar == '}') || (firstChar == '[' && lastChar == ']') {
		return true
	}

	// Check for JSON strings, numbers, booleans, null
	if firstChar == '"' ||
		strings.HasPrefix(trimmed, "true") ||
		strings.HasPrefix(trimmed, "false") ||
		strings.HasPrefix(trimmed, "null") ||
		(firstChar >= '0' && firstChar <= '9') || firstChar == '-' {
		return true
	}

	// YAML indicators that suggest it's NOT JSON
	// YAML documents often start with --- or have : without quotes
	if strings.HasPrefix(trimmed, "---") ||
		strings.Contains(trimmed, ":\n") ||
		strings.Contains(trimmed, ": ") && !strings.Contains(trimmed, "\": ") {
		return false
	}

	// Default to JSON for ambiguous cases
	return true
}

// StackedResolver combines multiple resolvers and tries them in order
type StackedResolver struct {
	resolvers []Resolver
}

// AddResolver adds a resolver to the stack
func (r *StackedResolver) AddResolver(resolver Resolver) {
	r.resolvers = append(r.resolvers, resolver)
}

// CanResolve returns true if any resolver can handle the resource, or always true for objectResolver fallback
func (r *StackedResolver) CanResolve(resource any) bool {
	// Check if any added resolver can handle it
	for _, resolver := range r.resolvers {
		if resolver.CanResolve(resource) {
			return true
		}
	}
	// objectResolver can attempt to handle anything as last resort
	return true
}

// Resolve resolves a JSON reference by trying each added resolver in order until one succeeds,
// with a built-in object resolver as the final fallback.
//
// This method follows the standard Resolver interface behavior:
//   - Uses the resource parameter as-is without modification
//   - Expects localRef to be a local reference starting with "#" (e.g., "#/path/to/data")
//   - Individual resolvers may return errors if the resource type doesn't match their expectations
//     (e.g., HTTP resolvers expect string URLs, file resolvers expect string paths)
//
// For convenience features like automatic full reference parsing
// (e.g., "https://example.com/data.json#/path"), use the global jsref.Resolve() function instead.
func (r *StackedResolver) Resolve(dst any, resource any, localRef string) error {
	var allErrors []error

	// Try added resolvers first
	for _, resolver := range r.resolvers {
		if resolver.CanResolve(resource) {
			err := resolver.Resolve(dst, resource, localRef)
			if err == nil {
				return nil
			}
			allErrors = append(allErrors, err)
		}
	}

	// As last resort, try objectResolver
	objResolver := objectResolver{}
	err := objResolver.Resolve(dst, resource, localRef)
	if err == nil {
		return nil
	}
	allErrors = append(allErrors, err)

	return fmt.Errorf("jsref: StackedResolver.Resolve: failed to resolve reource type %T. list of errors during resolution process: %w", resource, errors.Join(allErrors...))
}

// objectResolver resolves pointers against a single static object
type objectResolver struct{}

// New creates a new StackedResolver (empty, no default resolvers).
// Add specific resolvers using AddResolver() based on the resource types you need to handle.
func New() *StackedResolver {
	return &StackedResolver{resolvers: make([]Resolver, 0)}
}

// NewObjectResolver creates a new object resolver.
// This resolver accepts any resource type and attempts JSON pointer resolution against it.
// Use this resolver when your resource parameter in Resolve() will be data objects
// (maps, structs, slices, etc.) that you want to navigate with JSON pointers.
func NewObjectResolver() Resolver {
	return objectResolver{}
}

// CanResolve always returns true - objectResolver attempts to resolve against any resource
func (r objectResolver) CanResolve(resource any) bool {
	return true
}

// Resolve resolves a reference against the object
func (r objectResolver) Resolve(dst any, resource any, localRef string) error {
	// Local references must start with "#"
	if !strings.HasPrefix(localRef, "#") {
		return fmt.Errorf("jsref: objectResolver.Resolve: local references must start with '#', got: %s", localRef)
	}

	// Remove the "#" prefix to get the JSON pointer
	pointer := localRef[1:]
	ptr, err := jsptr.New(pointer)
	if err != nil {
		return fmt.Errorf("jsref: objectResolver.Resolve: invalid JSON pointer %s: %w", pointer, err)
	}
	return ptr.Retrieve(dst, resource)
}

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

// fsResolver resolves file-based references using os.Root
type fsResolver struct {
	root    *os.Root
	rootDir string
}

// NewFSResolver creates a new filesystem resolver rooted at the given directory.
// This resolver expects string resources containing file paths (relative to the root directory).
// Use this resolver when your resource parameter in Resolve() will be file path strings
// like "/path/to/data.json" or "config.yaml" that need to be loaded from the filesystem.
//
// If dir is empty, it defaults to the current directory.
// The resolver will only access files within the specified root directory for security.
func NewFSResolver(dir string) (Resolver, error) {
	if dir == "" {
		dir = "."
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("jsref.NewFSResolver: failed to open root directory %s: %w", dir, err)
	}
	return &fsResolver{root: root, rootDir: dir}, nil
}

// CanResolve returns true if the resource is a string that doesn't start with #
func (r *fsResolver) CanResolve(resource any) bool {
	str, ok := resource.(string)
	if !ok {
		return false
	}
	// Should not handle pure local references (starting with #)
	return !strings.HasPrefix(str, "#")
}

// Resolve resolves file references that may include fragments
func (r *fsResolver) Resolve(dst any, resource any, localRef string) error {
	str, ok := resource.(string)
	if !ok {
		return fmt.Errorf("jsref: fsResolver.Resolve: fsResolver requires string resource, got %T", resource)
	}
	// Should not handle pure local references (starting with #)
	if strings.HasPrefix(str, "#") {
		return fmt.Errorf("jsref: fsResolver.Resolve: fsResolver cannot handle local reference: %s", str)
	}

	filePath := str
	// Handle file:// URLs by extracting the path
	if u, err := url.Parse(filePath); err == nil {
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
				return fmt.Errorf("jsref: fsResolver.Resolve: failed to resolve root directory: %w", err)
			}
		}

		// Check if the absolute path is within our root
		relPath, err := filepath.Rel(rootDir, filePath)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return fmt.Errorf("jsref: fsResolver.Resolve: fs resolver cannot handle path outside its root: %s (root: %s)", filePath, rootDir)
		}
		filePath = relPath
	}

	// Clean the file path
	filePath = filepath.Clean(filePath)

	// Check if root was successfully opened
	if r.root == nil {
		return fmt.Errorf("jsref: fsResolver.Resolve: fs resolver root directory is not accessible: %s", r.rootDir)
	}

	// Read the file using os.Root
	file, err := r.root.Open(filePath)
	if err != nil {
		return fmt.Errorf("jsref: fsResolver.Resolve: failed to open file %s: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("jsref: fsResolver.Resolve: failed to read file %s: %w", filePath, err)
	}

	// Parse the data (try JSON first, then YAML)
	parsed, err := parseData(data)
	if err != nil {
		return fmt.Errorf("jsref: fsResolver.Resolve: failed to parse file %s: %w", filePath, err)
	}

	// Create an object resolver for the loaded data
	objectResolver := objectResolver{}

	// If no local reference provided, return the whole document
	if localRef == "" {
		// Use root reference
		return objectResolver.Resolve(dst, parsed, "#")
	}

	// Resolve with the local reference
	return objectResolver.Resolve(dst, parsed, localRef)
}

// globalResolver is a singleton StackedResolver for the global Resolve function
var globalResolver *StackedResolver

func init() {
	globalResolver = New()
	// Add HTTP resolver for URLs
	globalResolver.AddResolver(NewHTTPResolver())
	// Add FS resolver that allows access to any file location (root = "/")
	if fsResolver, err := NewFSResolver("/"); err == nil {
		globalResolver.AddResolver(fsResolver)
	}
}

// Resolve is a global convenience function that uses a stock StackedResolver.
//
// This function has the same signature as other Resolvers but with enhanced behavior:
//   - If localRef contains a full reference (e.g., "https://example.com/data.json#/path"),
//     it uses Split() to parse it and passes the correct values to the underlying StackedResolver,
//     ignoring the resource parameter entirely to avoid type mismatches with specific Resolvers
//   - If localRef is a pure local reference (e.g., "#/path"), it uses the resource parameter normally
//
// Examples:
//
//	jsref.Resolve(&dst, data, "#/path")                              // uses data as resource
//	jsref.Resolve(&dst, data, "/file.json#/path")                    // ignores data, uses "/file.json"
//	jsref.Resolve(&dst, nil, "https://example.com/data.json#/path")  // uses external URL
//
// This differs from StackedResolver.Resolve() which always uses the resource parameter as-is
// and may return errors if the resource type doesn't match what a specific Resolver expects.
func Resolve(dst any, resource any, localRef string) error {
	// Try to split the localRef to see if it contains a full reference
	external, local, err := Split(localRef)
	if err != nil {
		return fmt.Errorf("jsref.Resolve: failed to split reference: %w", err)
	}

	// If there's an external part, ignore the resource parameter and use the external part
	if external != "" {
		return globalResolver.Resolve(dst, external, local)
	}

	// No external part, use normal resolver behavior with provided resource
	if resource == nil {
		return fmt.Errorf("jsref.Resolve: cannot resolve pure local reference without a resource")
	}
	return globalResolver.Resolve(dst, resource, localRef)
}
