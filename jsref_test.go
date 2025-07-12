package jsref_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lestrrat-go/jsref/v2"
	"github.com/stretchr/testify/require"
)

func TestObjectResolver(t *testing.T) {
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

	resolver := jsref.New(data)

	t.Run("resolve root", func(t *testing.T) {
		var result map[string]any
		err := resolver.Resolve(&result, "#")
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("resolve name", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, "#/name")
		require.NoError(t, err)
		require.Equal(t, "John Doe", result)
	})

	t.Run("resolve nested address", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, "#/address/city")
		require.NoError(t, err)
		require.Equal(t, "New York", result)
	})

	t.Run("resolve array element", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, "#/hobbies/1")
		require.NoError(t, err)
		require.Equal(t, "coding", result)
	})

	t.Run("resolve nonexistent property", func(t *testing.T) {
		var result any
		err := resolver.Resolve(&result, "#/nonexistent")
		require.Error(t, err)
	})

	t.Run("reject reference without hash prefix", func(t *testing.T) {
		var result any
		err := resolver.Resolve(&result, "/name")
		require.Error(t, err)
		require.Contains(t, err.Error(), "local references must start with '#'")
	})
}

func TestFSResolver(t *testing.T) {
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

	resolver, err := jsref.NewFSResolver("")
	require.NoError(t, err)

	t.Run("resolve from JSON file - should fail with path outside root", func(t *testing.T) {
		var result string
		err = resolver.Resolve(&result, jsonFile+"#/database/host")
		require.Error(t, err)
		require.Contains(t, err.Error(), "fs resolver cannot handle path outside its root")
	})

	t.Run("resolve array from JSON file - should fail with path outside root", func(t *testing.T) {
		var result string
		err = resolver.Resolve(&result, jsonFile+"#/features/0")
		require.Error(t, err)
		require.Contains(t, err.Error(), "fs resolver cannot handle path outside its root")
	})

	t.Run("custom fs resolver with directory", func(t *testing.T) {
		// Create a resolver rooted at tmpDir
		customResolver, err := jsref.NewFSResolver(tmpDir)
		require.NoError(t, err)

		var result string
		// Use relative path since we're using tmpDir as the root
		err = customResolver.Resolve(&result, "test.json#/database/host")
		require.NoError(t, err)
		require.Equal(t, "localhost", result)
		
		// Test absolute path within root
		var result2 float64
		err = customResolver.Resolve(&result2, jsonFile+"#/database/port")
		require.NoError(t, err)
		require.Equal(t, float64(5432), result2)
	})

	t.Run("empty string defaults to current directory", func(t *testing.T) {
		emptyResolver, err := jsref.NewFSResolver("")
		require.NoError(t, err)
		// This should reject temp files outside the current directory
		var result string
		err = emptyResolver.Resolve(&result, jsonFile+"#/database/port")
		require.Error(t, err)
		require.Contains(t, err.Error(), "fs resolver cannot handle path outside its root")
	})
}

func TestStackedResolver(t *testing.T) {
	// Create test data for object resolver
	localData := map[string]any{
		"local": "value",
	}

	// Create temporary file for fs resolver
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "external.json")

	fileData := map[string]any{
		"external": "file-value",
	}

	jsonData, err := json.Marshal(fileData)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, jsonData, 0644)
	require.NoError(t, err)

	// Create stacked resolver using New() which now returns *StackedResolver
	stacked := jsref.New(localData)
	// Add fs resolver rooted at tmpDir for the external file
	tmpResolver, err := jsref.NewFSResolver(tmpDir)
	require.NoError(t, err)
	stacked.AddResolver(tmpResolver)

	t.Run("resolve from first resolver", func(t *testing.T) {
		var result string
		err := stacked.Resolve(&result, "#/local")
		require.NoError(t, err)
		require.Equal(t, "value", result)
	})

	t.Run("resolve from second resolver when first fails", func(t *testing.T) {
		var result string
		err := stacked.Resolve(&result, jsonFile+"#/external")
		require.NoError(t, err)
		require.Equal(t, "file-value", result)
	})

	t.Run("fail when no resolver can handle", func(t *testing.T) {
		var result any
		err := stacked.Resolve(&result, "#/nonexistent")
		require.Error(t, err)
	})
}

func TestFSResolverDynamic(t *testing.T) {
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

	// Create resolver rooted at tmpDir
	resolver, err := jsref.NewFSResolver(tmpDir)
	require.NoError(t, err)

	t.Run("resolve from file without fragment", func(t *testing.T) {
		var result map[string]any
		err = resolver.Resolve(&result, "dynamic.json")
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("resolve from file with fragment", func(t *testing.T) {
		var result float64
		err := resolver.Resolve(&result, "dynamic.json#/config/timeout")
		require.NoError(t, err)
		require.Equal(t, float64(30), result)
	})

	t.Run("resolve array element with fragment", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, "dynamic.json#/endpoints/1")
		require.NoError(t, err)
		require.Equal(t, "https://api.example.com/v2", result)
	})

	t.Run("resolve absolute path within root", func(t *testing.T) {
		var result float64
		err := resolver.Resolve(&result, jsonFile+"#/config/timeout")
		require.NoError(t, err)
		require.Equal(t, float64(30), result)
	})

	t.Run("local reference fails in fs resolver", func(t *testing.T) {
		var result any
		err := resolver.Resolve(&result, "#/config/timeout")
		require.Error(t, err)
		require.Contains(t, err.Error(), "fs resolver cannot handle local reference")
	})
}

func TestStackedResolverWithLocalAndDynamic(t *testing.T) {
	// Create test data for object resolver
	localData := map[string]any{
		"local": "local-value",
		"config": map[string]any{
			"retries": float64(3),
		},
	}

	// Create temporary file
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "dynamic.json")

	fileData := map[string]any{
		"external": "external-value",
	}

	jsonData, err := json.Marshal(fileData)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, jsonData, 0644)
	require.NoError(t, err)

	// Create stacked resolver with local data and fs resolver
	stacked := jsref.New(localData)
	tmpResolver, err := jsref.NewFSResolver(tmpDir)
	require.NoError(t, err)
	stacked.AddResolver(tmpResolver)

	t.Run("resolve local reference", func(t *testing.T) {
		var result string
		err := stacked.Resolve(&result, "#/local")
		require.NoError(t, err)
		require.Equal(t, "local-value", result)
	})

	t.Run("resolve external file reference", func(t *testing.T) {
		var result string
		err := stacked.Resolve(&result, "dynamic.json#/external")
		require.NoError(t, err)
		require.Equal(t, "external-value", result)
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

	resolver, err := jsref.NewFSResolver(tmpDir)
	require.NoError(t, err)

	t.Run("resolve from YAML file", func(t *testing.T) {
		var result string
		err = resolver.Resolve(&result, "test.yaml#/database/host")
		require.NoError(t, err)
		require.Equal(t, "localhost", result)
	})

	t.Run("resolve boolean from YAML", func(t *testing.T) {
		var result bool
		err = resolver.Resolve(&result, "test.yaml#/database/ssl")
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("resolve array element from YAML", func(t *testing.T) {
		var result string
		err = resolver.Resolve(&result, "test.yaml#/services/1/name")
		require.NoError(t, err)
		require.Equal(t, "api", result)
	})
}

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

	t.Run("object resolver with hash prefix", func(t *testing.T) {
		object := map[string]any{
			"name": "test",
			"data": map[string]any{
				"value": 42,
			},
		}

		r := jsref.New(object)

		var result string
		err := r.Resolve(&result, "#/name")
		require.NoError(t, err)
		require.Equal(t, "test", result)

		var result2 any
		err = r.Resolve(&result2, "#/data/value")
		require.NoError(t, err)
		require.Equal(t, 42, result2)
	})

	t.Run("fs resolver examples from spec", func(t *testing.T) {
		r, err := jsref.NewFSResolver(tmpDir)
		require.NoError(t, err)

		// r.Resolve(dst, "./path/to/object.json")
		var result1 map[string]any
		err = r.Resolve(&result1, "object.json")
		require.NoError(t, err)
		require.Equal(t, jsonData, result1)

		// r.Resolve(dst, "./path/to/object.json#/json/ptr")
		var result2 string
		err = r.Resolve(&result2, "object.json#/json/ptr")
		require.NoError(t, err)
		require.Equal(t, "pointer-value", result2)
	})

	t.Run("static fs resolver", func(t *testing.T) {
		r, err := jsref.NewFSResolver(tmpDir)
		require.NoError(t, err)

		var result string
		err = r.Resolve(&result, "object.json#/hello/world")
		require.NoError(t, err)
		require.Equal(t, "test-value", result)
	})

	t.Run("stacked resolver", func(t *testing.T) {
		localData := map[string]any{
			"local": "local-value",
		}

		stacked := jsref.New(localData)
		fsResolver, err := jsref.NewFSResolver(tmpDir)
		require.NoError(t, err)
		stacked.AddResolver(fsResolver)
		stacked.AddResolver(jsref.NewHTTPResolver())

		// Resolve local reference
		var result string
		err = stacked.Resolve(&result, "#/local")
		require.NoError(t, err)
		require.Equal(t, "local-value", result)

		// Resolve file reference
		err = stacked.Resolve(&result, "object.json#/hello/world")
		require.NoError(t, err)
		require.Equal(t, "test-value", result)
	})

	t.Run("error cases", func(t *testing.T) {
		r := jsref.New(map[string]any{"test": "value"})

		// Local reference without hash should fail
		var result any
		err := r.Resolve(&result, "/test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "local references must start with '#'")

		// HTTP resolver with local reference should fail
		hr := jsref.NewHTTPResolver()
		err = hr.Resolve(&result, "#/test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "HTTP resolver cannot handle local reference")
	})
}