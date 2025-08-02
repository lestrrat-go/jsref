package jsref

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// MyTestObject implements SelfResolver for testing
type MyTestObject struct {
	Foo string
	Bar int
}

// Resolve implements the SelfResolver interface
func (o MyTestObject) Resolve(ref string) (any, error) {
	switch ref {
	case "foo":
		return o.Foo, nil
	case "bar":
		return o.Bar, nil
	default:
		return nil, fmt.Errorf("cannot resolve %s", ref)
	}
}

// TestSelfResolver tests the SelfResolver interface implementation
func TestSelfResolver(t *testing.T) {
	obj := MyTestObject{Foo: "hello", Bar: 42}
	resolver := NewObjectResolver()

	t.Run("resolve using SelfResolver interface", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/foo")
		require.NoError(t, err)
		require.Equal(t, "hello", result)

		var intResult int
		err = resolver.Resolve(&intResult, obj, "#/bar")
		require.NoError(t, err)
		require.Equal(t, 42, intResult)
	})

	t.Run("SelfResolver error handling", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot resolve nonexistent")
	})
}

// TestMapResolution tests map resolution with different value types
func TestMapResolution(t *testing.T) {
	resolver := NewObjectResolver()

	t.Run("map[string]any", func(t *testing.T) {
		obj := map[string]any{
			"foo": "value1",
			"bar": 123,
			"nested": map[string]any{
				"inner": "deep",
			},
		}

		var result string
		err := resolver.Resolve(&result, obj, "#/foo")
		require.NoError(t, err)
		require.Equal(t, "value1", result)

		var intResult int
		err = resolver.Resolve(&intResult, obj, "#/bar")
		require.NoError(t, err)
		require.Equal(t, 123, intResult)

		var nestedResult string
		err = resolver.Resolve(&nestedResult, obj, "#/nested/inner")
		require.NoError(t, err)
		require.Equal(t, "deep", nestedResult)
	})

	t.Run("map[string]string", func(t *testing.T) {
		obj := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		var result string
		err := resolver.Resolve(&result, obj, "#/key1")
		require.NoError(t, err)
		require.Equal(t, "value1", result)
	})

	t.Run("map with non-string keys should fail", func(t *testing.T) {
		obj := map[int]string{
			1: "value1",
		}

		var result string
		err := resolver.Resolve(&result, obj, "#/1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "map keys must be strings")
	})

	t.Run("nonexistent key", func(t *testing.T) {
		obj := map[string]string{
			"existing": "value",
		}

		var result string
		err := resolver.Resolve(&result, obj, "#/nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "key 'nonexistent' not found")
	})
}

// TestSliceArrayResolution tests slice and array resolution
func TestSliceArrayResolution(t *testing.T) {
	resolver := NewObjectResolver()

	t.Run("slice of strings", func(t *testing.T) {
		obj := []string{"first", "second", "third"}

		var result string
		err := resolver.Resolve(&result, obj, "#/0")
		require.NoError(t, err)
		require.Equal(t, "first", result)

		err = resolver.Resolve(&result, obj, "#/2")
		require.NoError(t, err)
		require.Equal(t, "third", result)
	})

	t.Run("slice of any", func(t *testing.T) {
		obj := []any{"string", 42, true}

		var strResult string
		err := resolver.Resolve(&strResult, obj, "#/0")
		require.NoError(t, err)
		require.Equal(t, "string", strResult)

		var intResult int
		err = resolver.Resolve(&intResult, obj, "#/1")
		require.NoError(t, err)
		require.Equal(t, 42, intResult)

		var boolResult bool
		err = resolver.Resolve(&boolResult, obj, "#/2")
		require.NoError(t, err)
		require.Equal(t, true, boolResult)
	})

	t.Run("nested slice access", func(t *testing.T) {
		obj := [][]string{
			{"a", "b"},
			{"c", "d"},
		}

		var result string
		err := resolver.Resolve(&result, obj, "#/1/0")
		require.NoError(t, err)
		require.Equal(t, "c", result)
	})

	t.Run("array bounds error", func(t *testing.T) {
		obj := []string{"first", "second"}

		var result string
		err := resolver.Resolve(&result, obj, "#/5")
		require.Error(t, err)
		require.Contains(t, err.Error(), "index 5 out of bounds")
	})

	t.Run("invalid index", func(t *testing.T) {
		obj := []string{"first", "second"}

		var result string
		err := resolver.Resolve(&result, obj, "#/abc")
		require.Error(t, err)
		require.Contains(t, err.Error(), "slice/array index must be integer")
	})

	t.Run("negative index", func(t *testing.T) {
		obj := []string{"first", "second"}

		var result string
		err := resolver.Resolve(&result, obj, "#/-1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "index -1 out of bounds")
	})
}

// TestStructResolution tests struct resolution with JSON tags and field names
func TestStructResolution(t *testing.T) {
	type TestStruct struct {
		Name        string `json:"name"`
		Age         int    `json:"age,omitempty"`
		DirectField string
		unexported  string // This should not be accessible
		Nested      struct {
			Value string `json:"value"`
		} `json:"nested"`
	}

	obj := TestStruct{
		Name:        "John",
		Age:         30,
		DirectField: "direct",
		unexported:  "secret",
		Nested: struct {
			Value string `json:"value"`
		}{Value: "nested_value"},
	}

	resolver := NewObjectResolver()

	t.Run("resolve by JSON tag", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/name")
		require.NoError(t, err)
		require.Equal(t, "John", result)

		var intResult int
		err = resolver.Resolve(&intResult, obj, "#/age")
		require.NoError(t, err)
		require.Equal(t, 30, intResult)
	})

	t.Run("resolve by field name when no JSON tag", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/DirectField")
		require.NoError(t, err)
		require.Equal(t, "direct", result)
	})

	t.Run("cannot access unexported field", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/unexported")
		require.Error(t, err)
		require.Contains(t, err.Error(), "field 'unexported' not found")
	})

	t.Run("nested struct access", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/nested/value")
		require.NoError(t, err)
		require.Equal(t, "nested_value", result)
	})

	t.Run("nonexistent field", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "field 'nonexistent' not found")
	})
}

// TestPointerHandling tests resolution through pointer types
func TestPointerHandling(t *testing.T) {
	type TestStruct struct {
		Value string `json:"value"`
	}

	obj := &TestStruct{Value: "test"}
	resolver := NewObjectResolver()

	t.Run("resolve through pointer", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, obj, "#/value")
		require.NoError(t, err)
		require.Equal(t, "test", result)
	})

	t.Run("nil pointer should error", func(t *testing.T) {
		var nilPtr *TestStruct
		var result string
		err := resolver.Resolve(&result, nilPtr, "#/value")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot resolve segment 'value' on nil pointer")
	})
}

// TestInterfaceHandling tests resolution through interface{} types
func TestInterfaceHandling(t *testing.T) {
	resolver := NewObjectResolver()

	t.Run("resolve through interface{}", func(t *testing.T) {
		var obj any = map[string]string{"key": "value"}

		var result string
		err := resolver.Resolve(&result, obj, "#/key")
		require.NoError(t, err)
		require.Equal(t, "value", result)
	})

	t.Run("nil interface should error", func(t *testing.T) {
		var nilInterface any
		var result string
		err := resolver.Resolve(&result, nilInterface, "#/key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot resolve segment 'key' on nil object")
	})
}

// TestRootReference tests the root reference "#"
func TestRootReference(t *testing.T) {
	resolver := NewObjectResolver()
	obj := map[string]string{"key": "value"}

	var result map[string]string
	err := resolver.Resolve(&result, obj, "#")
	require.NoError(t, err)
	require.Equal(t, obj, result)
}

// TestUnsupportedTypes tests error handling for unsupported types
func TestUnsupportedTypes(t *testing.T) {
	resolver := NewObjectResolver()

	t.Run("unsupported type", func(t *testing.T) {
		obj := func() string { return "function" }

		var result string
		err := resolver.Resolve(&result, obj, "#/something")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot resolve segment 'something' on type")
	})

	t.Run("nil object", func(t *testing.T) {
		var result string
		err := resolver.Resolve(&result, nil, "#/something")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot resolve segment 'something' on nil object")
	})
}