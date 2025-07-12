# github.com/lestrrat-go/jsref/v2

This module provides ways to handle JSON References gracefully.

<!-- INCLUDE(./jsref_example_test.go) -->
```go
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

  r := jsref.New(data)
  r.AddResolver(jsref.NewDynamicResolver())

  for _, ref := range []string{
    // Local reference to the nested message
    "#/deep/nested/message",
    // File reference to the message in the temporary file
    fmt.Sprintf("file://%s#/deep/nested/message", f.Name()),
    // Remote reference to the message served by the test server
    fmt.Sprintf("%s#/message", s.URL),
  } {
    var dst string
    if err := r.Resolve(&dst, ref); err != nil {
      fmt.Printf("failed to resolve reference %s: %s\n", ref, err)
      return
    }

    if dst != "hello, world" {
      fmt.Printf("expected 'hello, world', got '%s'\n", dst)
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
```
source: [./jsref_example_test.go](https://github.com/lestrrat-go/jsref/blob/v2/./jsref_example_test.go)
<!-- END INCLUDE -->
