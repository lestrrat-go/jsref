package jsref_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

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

	r := jsref.New()
	r.AddResolver(jsref.NewHTTPResolver())
	
	// Create fs resolver that can handle the temp file's directory
	tmpDir := filepath.Dir(f.Name())
	fsResolver, err := jsref.NewFSResolver(tmpDir)
	if err != nil {
		fmt.Printf("failed to create fs resolver: %s\n", err)
		return
	}
	r.AddResolver(fsResolver)

	testCases := []struct{
		resource any
		localRef string
		expected string
	}{
		// Local reference to the nested message using objectResolver fallback
		{data, "#/deep/nested/message", "hello, world"},
		// File reference to the message in the temporary file
		{filepath.Base(f.Name()), "#/deep/nested/message", "hello, world"},
		// Remote reference to the message served by the test server
		{s.URL, "#/message", "hello, world"},
	}
	
	for _, tc := range testCases {
		var dst string
		if err := r.Resolve(&dst, tc.resource, tc.localRef); err != nil {
			fmt.Printf("failed to resolve reference: %s\n", err)
			return
		}

		if dst != tc.expected {
			fmt.Printf("expected '%s', got '%s'\n", tc.expected, dst)
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
