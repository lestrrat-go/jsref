package jsref

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

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
