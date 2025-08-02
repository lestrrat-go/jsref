package jsref

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
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

// SelfResolver interface allows objects to define their own resolution logic.
// Objects implementing this interface can handle reference resolution internally
// rather than relying on reflection-based field access.
type SelfResolver interface {
	// Resolve receives a reference fragment (like "foo") and returns the corresponding value.
	// This allows objects to customize how their fields/properties are accessed during
	// JSON reference resolution.
	Resolve(string) (any, error)
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

// New creates a new StackedResolver (empty, no default resolvers).
// Add specific resolvers using AddResolver() based on the resource types you need to handle.
func New() *StackedResolver {
	return &StackedResolver{resolvers: make([]Resolver, 0)}
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
