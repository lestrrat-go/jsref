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

// Person demonstrates the SelfResolver interface for custom resolution logic
type Person struct {
	Name string
	Age  int
}

// Resolve implements jsref.SelfResolver
func (p Person) Resolve(ref string) (any, error) {
	switch ref {
	case "name":
		return p.Name, nil
	case "age":
		return p.Age, nil
	case "isAdult":
		return p.Age >= 18, nil
	default:
		return nil, fmt.Errorf("unknown field: %s", ref)
	}
}

func Example_self_resolver() {
	person := Person{Name: "Alice", Age: 25}

	var name string
	err := jsref.Resolve(&name, person, "#/name")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Name: %s\n", name)

	var isAdult bool
	err = jsref.Resolve(&isAdult, person, "#/isAdult")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Is adult: %t\n", isAdult)

	// OUTPUT:
	// Name: Alice
	// Is adult: true
}

func Example_map_resolution() {
	data := map[string]any{
		"users": []map[string]any{
			{"name": "Bob", "age": 30},
			{"name": "Carol", "age": 28},
		},
		"config": map[string]any{
			"debug": true,
		},
	}

	var userName string
	err := jsref.Resolve(&userName, data, "#/users/1/name")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Second user: %s\n", userName)

	var debugMode bool
	err = jsref.Resolve(&debugMode, data, "#/config/debug")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Debug mode: %t\n", debugMode)

	// OUTPUT:
	// Second user: Carol
	// Debug mode: true
}

// Employee demonstrates struct resolution with JSON tags
type Employee struct {
	ID       int    `json:"id"`
	Name     string `json:"full_name"`
	Position string
}

func Example_struct_with_json_tags() {
	employee := Employee{
		ID:       123,
		Name:     "David",
		Position: "Engineer",
	}

	var empID int
	err := jsref.Resolve(&empID, employee, "#/id")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Employee ID: %d\n", empID)

	var fullName string
	err = jsref.Resolve(&fullName, employee, "#/full_name")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Full name: %s\n", fullName)

	var position string
	err = jsref.Resolve(&position, employee, "#/Position")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Position: %s\n", position)

	// OUTPUT:
	// Employee ID: 123
	// Full name: David
	// Position: Engineer
}

func Example_slice_resolution() {
	data := []any{"first", 42, true, map[string]string{"nested": "value"}}

	var str string
	err := jsref.Resolve(&str, data, "#/0")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("First element: %s\n", str)

	var num int
	err = jsref.Resolve(&num, data, "#/1")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Second element: %d\n", num)

	var nested string
	err = jsref.Resolve(&nested, data, "#/3/nested")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Nested value: %s\n", nested)

	// OUTPUT:
	// First element: first
	// Second element: 42
	// Nested value: value
}
