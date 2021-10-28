package jsref

import "github.com/lestrrat-go/option"

type Option = option.Interface

type identRecursiveResolution struct{}

// WithRecursiveResolution allows ou to enable recursive resolution
// on the *result* data structure. This means that after resolving
// the JSON reference in the structure at hand, it does another
// pass at resolving the entire data structure. Depending on your
// structure and size, this may incur significant cost.
//
// Please note that recursive resolution of the result is still
// experimental. If you find problems, please submit a pull request
// with a failing test case.
func WithRecursiveResolution(b bool) Option {
	return option.New(identRecursiveResolution{}, b)
}
