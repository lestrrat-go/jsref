package jsref

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/lestrrat-go/blackmagic"
)

// objectResolver resolves pointers against a single static object
type objectResolver struct{}

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

// Resolve resolves a reference against the object using the sophisticated object handling
// described in the specification. This includes support for SelfResolver interface,
// maps, slices/arrays, and struct reflection with JSON tags.
func (r objectResolver) Resolve(dst any, resource any, localRef string) error {
	// Local references must start with "#"
	if !strings.HasPrefix(localRef, "#") {
		return fmt.Errorf("jsref: objectResolver.Resolve: local references must start with '#', got: %s", localRef)
	}

	// Remove the "#" prefix to get the JSON pointer
	pointer := localRef[1:]
	
	// Split the pointer into path segments
	segments := strings.Split(pointer, "/")
	if len(segments) == 1 && segments[0] == "" {
		// Root reference "#" - return the whole object
		return blackmagic.AssignIfCompatible(dst, resource)
	}

	// Navigate through the path segments
	current := resource
	for _, segment := range segments {
		if segment == "" {
			continue // Skip empty segments (like leading slash)
		}
		
		var err error
		current, err = r.resolveSegment(current, segment)
		if err != nil {
			return fmt.Errorf("jsref: objectResolver.Resolve: failed to resolve segment '%s': %w", segment, err)
		}
	}

	return blackmagic.AssignIfCompatible(dst, current)
}

// resolveSegment resolves a single path segment against an object
func (r objectResolver) resolveSegment(obj any, segment string) (any, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot resolve segment '%s' on nil object", segment)
	}

	// First check if the object implements SelfResolver
	if selfResolver, ok := obj.(SelfResolver); ok {
		return selfResolver.Resolve(segment)
	}

	// Use reflection to handle different types
	val := reflect.ValueOf(obj)
	
	// Handle pointers by dereferencing them
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("cannot resolve segment '%s' on nil pointer", segment)
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Map:
		return r.resolveMapSegment(val, segment)
	case reflect.Slice, reflect.Array:
		return r.resolveSliceSegment(val, segment)
	case reflect.Struct:
		return r.resolveStructSegment(val, segment)
	case reflect.Interface:
		// If it's an interface{}, get the underlying value
		if !val.IsNil() {
			return r.resolveSegment(val.Interface(), segment)
		}
		return nil, fmt.Errorf("cannot resolve segment '%s' on nil interface", segment)
	default:
		return nil, fmt.Errorf("cannot resolve segment '%s' on type %s", segment, val.Type())
	}
}

// resolveMapSegment resolves a segment against a map
func (r objectResolver) resolveMapSegment(mapVal reflect.Value, segment string) (any, error) {
	// Maps must have string keys
	keyType := mapVal.Type().Key()
	if keyType.Kind() != reflect.String {
		return nil, fmt.Errorf("map keys must be strings, got %s", keyType)
	}

	key := reflect.ValueOf(segment)
	value := mapVal.MapIndex(key)
	if !value.IsValid() {
		return nil, fmt.Errorf("key '%s' not found in map", segment)
	}

	return value.Interface(), nil
}

// resolveSliceSegment resolves a segment against a slice or array
func (r objectResolver) resolveSliceSegment(sliceVal reflect.Value, segment string) (any, error) {
	// Parse segment as integer index
	index, err := strconv.Atoi(segment)
	if err != nil {
		return nil, fmt.Errorf("slice/array index must be integer, got '%s'", segment)
	}

	if index < 0 || index >= sliceVal.Len() {
		return nil, fmt.Errorf("index %d out of bounds for slice/array of length %d", index, sliceVal.Len())
	}

	return sliceVal.Index(index).Interface(), nil
}

// resolveStructSegment resolves a segment against a struct using reflection
func (r objectResolver) resolveStructSegment(structVal reflect.Value, segment string) (any, error) {
	structType := structVal.Type()

	// First try to find by JSON tag
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		
		// Only consider exported fields
		if !field.IsExported() {
			continue
		}

		// Check JSON tag
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			// Parse JSON tag (handle "fieldname,omitempty" format)
			tagName := strings.Split(jsonTag, ",")[0]
			if tagName == segment {
				return structVal.Field(i).Interface(), nil
			}
		}
	}

	// If no JSON tag match, try field name directly
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		
		// Only consider exported fields
		if !field.IsExported() {
			continue
		}

		if field.Name == segment {
			return structVal.Field(i).Interface(), nil
		}
	}

	return nil, fmt.Errorf("field '%s' not found in struct %s", segment, structType)
}
