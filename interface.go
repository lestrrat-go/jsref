package jsref

import (
	"net/url"
	"reflect"
)

var zeroval = reflect.Value{}

// Resolver is responsible for interpreting the provided JSON
// reference.
type Resolver struct {
	providers []Provider
}

// Provider resolves a URL into a ... thing.
type Provider interface {
	Get(*url.URL) (interface{}, error)
}
