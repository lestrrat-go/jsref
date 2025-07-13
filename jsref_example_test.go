package jsref_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/lestrrat-go/jsref/v2"
)

func Example() {
	s, cancel := startServer()
	defer cancel()

	data := map[string]any{
		"deep": map[string]any{
			"nested": map[string]any{
				"message": "hello, world",
			},
		},
	}

	// Create a temporary file to simulate a file-based JSON reference
	f, err := os.CreateTemp("", "jsref_example_*.json")
	if err != nil {
		fmt.Printf("failed to create temp file: %s\n", err)
		return
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if err := json.NewEncoder(f).Encode(data); err != nil {
		fmt.Printf("failed to write data to temp file: %s\n", err)
		return
	}

	// Use the global jsref.Resolve function - it handles HTTP, file, and local references automatically
	testCases := []struct {
		name     string
		resource any
		localRef string
		expected string
	}{
		// Local reference to the message in the data structure
		{"local data", data, "#/deep/nested/message", "hello, world"},
		// File reference to the message in the temporary file
		{"file resource", f.Name(), "#/deep/nested/message", "hello, world"},
		// Remote reference to the message served by the test server
		{"http resource", s.URL, "#/message", "hello, world"},
		// Full reference with nil resource (convenience mode)
		{"full file reference with nil", nil, f.Name() + "#/deep/nested/message", "hello, world"},
		// Full HTTP reference with nil resource (convenience mode)
		{"full http reference with nil", nil, s.URL + "#/message", "hello, world"},
		// Full reference with non-nil resource (resource gets ignored)
		{"full file reference ignores resource", data, f.Name() + "#/deep/nested/message", "hello, world"},
		// Full HTTP reference with non-nil resource (resource gets ignored)
		{"full http reference ignores resource", data, s.URL + "#/message", "hello, world"},
	}

	for _, tc := range testCases {
		var dst string
		if err := jsref.Resolve(&dst, tc.resource, tc.localRef); err != nil {
			fmt.Printf("failed to resolve %s: %s\n", tc.name, err)
			return
		}

		if dst != tc.expected {
			fmt.Printf("expected '%s', got '%s' for %s\n", tc.expected, dst, tc.name)
			return
		}
	}

	// OUTPUT:
}

func startServer() (*httptest.Server, func()) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Handle the request here
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"message": "hello, world"}`))
	}))

	return s, s.Close
}
