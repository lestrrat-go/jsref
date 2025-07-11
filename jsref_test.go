package jsref_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lestrrat-go/jsref/v2"
	"github.com/stretchr/testify/require"
)

func TestLocalResolver(t *testing.T) {
	// Create test data
	data := map[string]any{
		"name": "John Doe",
		"age":  30,
		"address": map[string]any{
			"street": "123 Main St",
			"city":   "New York",
		},
		"hobbies": []any{"reading", "coding", "hiking"},
	}

	ctx := context.Background()
	resolver := jsref.New(data)

	t.Run("resolve root", func(t *testing.T) {
		var result map[string]any
		err := resolver.Resolve(ctx, &result, "#")
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("resolve name", func(t *testing.T) {
		var result string
		err := resolver.Resolve(ctx, &result, "#/name")
		require.NoError(t, err)
		require.Equal(t, "John Doe", result)
	})

	t.Run("resolve nested address", func(t *testing.T) {
		var result string
		err := resolver.Resolve(ctx, &result, "#/address/city")
		require.NoError(t, err)
		require.Equal(t, "New York", result)
	})

	t.Run("resolve array element", func(t *testing.T) {
		var result string
		err := resolver.Resolve(ctx, &result, "#/hobbies/1")
		require.NoError(t, err)
		require.Equal(t, "coding", result)
	})

	t.Run("resolve nonexistent property", func(t *testing.T) {
		var result any
		err := resolver.Resolve(ctx, &result, "#/nonexistent")
		require.Error(t, err)
	})

	t.Run("reject reference without hash prefix", func(t *testing.T) {
		var result any
		err := resolver.Resolve(ctx, &result, "/name")
		require.Error(t, err)
		require.Contains(t, err.Error(), "local references must start with '#'")
	})
}

func TestFileResolver(t *testing.T) {
	// Create temporary JSON file
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")

	data := map[string]any{
		"database": map[string]any{
			"host": "localhost",
			"port": 5432,
		},
		"features": []any{"auth", "api", "storage"},
	}

	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, jsonData, 0644)
	require.NoError(t, err)

	ctx := context.Background()
	resolver := jsref.NewFileResolver(jsonFile)

	t.Run("resolve from JSON file", func(t *testing.T) {
		var result string
		err := resolver.Resolve(ctx, &result, "#/database/host")
		require.NoError(t, err)
		require.Equal(t, "localhost", result)
	})

	t.Run("resolve array from JSON file", func(t *testing.T) {
		var result string
		err := resolver.Resolve(ctx, &result, "#/features/0")
		require.NoError(t, err)
		require.Equal(t, "auth", result)
	})
}

func TestStackedResolver(t *testing.T) {
	// Create test data for local resolver
	localData := map[string]any{
		"local": "value",
	}

	// Create temporary file for file resolver
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "external.json")

	fileData := map[string]any{
		"external": "file-value",
	}

	jsonData, err := json.Marshal(fileData)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, jsonData, 0644)
	require.NoError(t, err)

	ctx := context.Background()

	// Create stacked resolver
	stacked := jsref.NewStackedResolver()
	stacked.AddResolver(jsref.New(localData))
	stacked.AddResolver(jsref.NewFileResolver(jsonFile))

	t.Run("resolve from first resolver", func(t *testing.T) {
		var result string
		err := stacked.Resolve(ctx, &result, "#/local")
		require.NoError(t, err)
		require.Equal(t, "value", result)
	})

	t.Run("resolve from second resolver when first fails", func(t *testing.T) {
		var result string
		err := stacked.Resolve(ctx, &result, "#/external")
		require.NoError(t, err)
		require.Equal(t, "file-value", result)
	})

	t.Run("fail when no resolver can handle", func(t *testing.T) {
		var result any
		err := stacked.Resolve(ctx, &result, "#/nonexistent")
		require.Error(t, err)
	})
}

func TestDynamicResolver(t *testing.T) {
	// Create temporary JSON file
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "dynamic.json")

	data := map[string]any{
		"config": map[string]any{
			"timeout": float64(30), // JSON numbers are float64
			"retries": float64(3),
		},
		"endpoints": []any{
			"https://api.example.com/v1",
			"https://api.example.com/v2",
		},
	}

	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, jsonData, 0644)
	require.NoError(t, err)

	ctx := context.Background()
	resolver := jsref.NewDynamicResolver()

	t.Run("resolve from file without fragment", func(t *testing.T) {
		var result map[string]any
		err = resolver.Resolve(ctx, &result, jsonFile)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("resolve from file with fragment", func(t *testing.T) {
		reference := fmt.Sprintf("%s#/config/timeout", jsonFile)
		var result float64
		err = resolver.Resolve(ctx, &result, reference)
		require.NoError(t, err)
		require.Equal(t, float64(30), result)
	})

	t.Run("resolve array element with fragment", func(t *testing.T) {
		reference := fmt.Sprintf("%s#/endpoints/1", jsonFile)
		var result string
		err = resolver.Resolve(ctx, &result, reference)
		require.NoError(t, err)
		require.Equal(t, "https://api.example.com/v2", result)
	})

	t.Run("resolve local reference with context", func(t *testing.T) {
		// Set up context with a resolver
		localResolver := jsref.New(data)
		ctxWithResolver := jsref.WithResolver(ctx, localResolver)

		var result any // Use any instead of float64 to handle type conversion
		err := resolver.Resolve(ctxWithResolver, &result, "#/config/retries")
		require.NoError(t, err)
		require.Equal(t, float64(3), result)
	})

	t.Run("fail local reference without context", func(t *testing.T) {
		var result any
		err := resolver.Resolve(ctx, &result, "#/config/timeout")
		require.Error(t, err)
		require.Contains(t, err.Error(), "local reference requires a resolver in context")
	})
}

func TestContextFunctions(t *testing.T) {
	data := map[string]any{"test": "value"}
	resolver := jsref.New(data)

	ctx := context.Background()

	t.Run("resolver not in context", func(t *testing.T) {
		_, exists := jsref.FromContext(ctx)
		require.False(t, exists)
	})

	t.Run("resolver in context", func(t *testing.T) {
		ctxWithResolver := jsref.WithResolver(ctx, resolver)
		retrievedResolver, exists := jsref.FromContext(ctxWithResolver)
		require.True(t, exists)
		require.Equal(t, resolver, retrievedResolver)
	})
}

func TestYAMLSupport(t *testing.T) {
	// Create temporary YAML file
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
name: "Test Config"
database:
  host: localhost
  port: 5432
  ssl: true
services:
  - name: "web"
    port: 8080
  - name: "api"
    port: 3000
`

	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	resolver := jsref.NewDynamicResolver()

	t.Run("resolve from YAML file", func(t *testing.T) {
		reference := fmt.Sprintf("%s#/database/host", yamlFile)
		var result string
		err = resolver.Resolve(ctx, &result, reference)
		require.NoError(t, err)
		require.Equal(t, "localhost", result)
	})

	t.Run("resolve boolean from YAML", func(t *testing.T) {
		reference := fmt.Sprintf("%s#/database/ssl", yamlFile)
		var result bool
		err = resolver.Resolve(ctx, &result, reference)
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("resolve array element from YAML", func(t *testing.T) {
		reference := fmt.Sprintf("%s#/services/1/name", yamlFile)
		var result string
		err = resolver.Resolve(ctx, &result, reference)
		require.NoError(t, err)
		require.Equal(t, "api", result)
	})
}

// TestSpecificationExamples tests the exact examples from the specification
func TestSpecificationExamples(t *testing.T) {
	// Create test files as per spec examples
	tmpDir := t.TempDir()

	// Create object.json
	jsonFile := filepath.Join(tmpDir, "object.json")
	jsonData := map[string]any{
		"hello": map[string]any{
			"world": "test-value",
		},
		"json": map[string]any{
			"ptr": "pointer-value",
		},
	}
	jsonBytes, err := json.Marshal(jsonData)
	require.NoError(t, err)
	err = os.WriteFile(jsonFile, jsonBytes, 0644)
	require.NoError(t, err)

	// Create object.yaml
	yamlFile := filepath.Join(tmpDir, "object.yaml")
	yamlContent := `
hello:
  world: "yaml-value"
json:
  ptr: "yaml-pointer-value"
`
	err = os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("local resolver with hash prefix", func(t *testing.T) {
		object := map[string]any{
			"name": "test",
			"data": map[string]any{
				"value": 42,
			},
		}

		r := jsref.New(object)

		var result string
		err := r.Resolve(ctx, &result, "#/name")
		require.NoError(t, err)
		require.Equal(t, "test", result)

		var result2 any
		err = r.Resolve(ctx, &result2, "#/data/value")
		require.NoError(t, err)
		require.Equal(t, 42, result2)
	})

	t.Run("dynamic resolver examples from spec", func(t *testing.T) {
		r := jsref.NewDynamicResolver()

		// r.Resolve(ctx, dst, "./path/to/object.json")
		var result1 map[string]any
		err := r.Resolve(ctx, &result1, jsonFile)
		require.NoError(t, err)
		require.Equal(t, jsonData, result1)

		// r.Resolve(ctx, dst, "./path/to/object.yaml#/json/ptr")
		var result2 string
		err = r.Resolve(ctx, &result2, yamlFile+"#/json/ptr")
		require.NoError(t, err)
		require.Equal(t, "yaml-pointer-value", result2)

		// Test without fragment returns whole document
		var result3 map[string]any
		err = r.Resolve(ctx, &result3, yamlFile)
		require.NoError(t, err)
		require.Contains(t, result3, "hello")
		require.Contains(t, result3, "json")
	})

	t.Run("static file resolver", func(t *testing.T) {
		r := jsref.NewFileResolver(jsonFile)

		var result string
		err := r.Resolve(ctx, &result, "#/hello/world")
		require.NoError(t, err)
		require.Equal(t, "test-value", result)
	})

	t.Run("stacked resolver with context", func(t *testing.T) {
		localData := map[string]any{
			"local": "local-value",
		}

		stackedResolver := jsref.NewStackedResolver()
		stackedResolver.AddResolver(jsref.NewFileResolver(jsonFile))
		stackedResolver.AddResolver(jsref.NewDynamicResolver())

		// Use context to pass stacked resolver for local references
		ctxWithResolver := jsref.WithResolver(ctx, stackedResolver)
		r := jsref.New(localData)

		// Resolve local reference using context resolver
		var result string
		err := r.Resolve(ctxWithResolver, &result, "#/local")
		require.NoError(t, err)
		require.Equal(t, "local-value", result)
	})

	t.Run("error cases", func(t *testing.T) {
		r := jsref.New(map[string]any{"test": "value"})

		// Local reference without hash should fail
		var result any
		err := r.Resolve(ctx, &result, "/test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "local references must start with '#'")

		// Dynamic resolver with local reference but no context should fail
		dr := jsref.NewDynamicResolver()
		err = dr.Resolve(ctx, &result, "#/test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "local reference requires a resolver in context")
	})
}
