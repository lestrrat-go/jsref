package jsref

import (
	"fmt"
	"strings"

	"github.com/lestrrat-go/jsptr"
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

// Resolve resolves a reference against the object
func (r objectResolver) Resolve(dst any, resource any, localRef string) error {
	// Local references must start with "#"
	if !strings.HasPrefix(localRef, "#") {
		return fmt.Errorf("jsref: objectResolver.Resolve: local references must start with '#', got: %s", localRef)
	}

	// Remove the "#" prefix to get the JSON pointer
	pointer := localRef[1:]
	ptr, err := jsptr.New(pointer)
	if err != nil {
		return fmt.Errorf("jsref: objectResolver.Resolve: invalid JSON pointer %s: %w", pointer, err)
	}
	return ptr.Retrieve(dst, resource)
}
