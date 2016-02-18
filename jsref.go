package jsref

import (
	"errors"
	"net/url"
	"reflect"

	"github.com/lestrrat/go-jspointer"
	"github.com/lestrrat/go-pdebug"
	"github.com/lestrrat/go-structinfo"
)

// New creates a new Resolver
func New() *Resolver {
	return &Resolver{}
}

// AddProvider adds a new Provider to be searched for in case
// a JSON pointer with more than just the URI fragment is given
func (r *Resolver) AddProvider(p Provider) error {
	r.providers = append(r.providers, p)
	return nil
}

// Resolve takes a target `v`, and a JSON pointer `spec`.
// spec is expected to be in the form of
//
//    [scheme://[userinfo@]host/path[?query]]#fragment
//    [scheme:opaque[?query]]#fragment
//
// where everything except for `#fragment` is optional.
//
// If `spec` is the empty string, `v` is returned
// This method handles recursive JSON references.
func (r *Resolver) Resolve(v interface{}, spec string) (ret interface{}, err error) {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START Resolver.Resolve(%s)", spec)
		defer func() {
			if err != nil {
				g.IRelease("END Resolver.Resolve(%s): %s", spec, err)
			} else {
				g.IRelease("END Resolver.Resolve(%s)", spec)
			}
		}()
	}

	if spec == "" {
		return v, nil
	}

	u, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}

	for _, p := range r.providers {
		pv, err := p.Get(u)
		if err == nil {
			return r.Resolve(pv, "#"+u.Fragment)
		}
	}

	x, err := matchjsp(v, u.Fragment)
	if err != nil {
		return nil, err
	}

	rv := reflect.ValueOf(x)
	if rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map:
		refv := rv.MapIndex(reflect.ValueOf("$ref"))
		return recurse(r, v, refv)
	case reflect.Struct:
		i := structinfo.StructFieldFromJSONName(rv, "$ref")
		refv := rv.Field(i)
		return recurse(r, v, refv)
	}

	return x, nil
}

func matchjsp(v interface{}, ptr string) (interface{}, error) {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START matchjsp(%s)", ptr)
		defer g.IRelease("END matchjsp(%s)", ptr)
	}
	if ptr == "" {
		return v, nil
	}

	p, err := jspointer.New(ptr)
	if err != nil {
		return nil, err
	}

	return p.Get(v)
}

func recurse(r *Resolver, x interface{}, refv reflect.Value) (interface{}, error) {
	if refv.Kind() == reflect.Interface {
		refv = refv.Elem()
	}

	switch refv.Kind() {
	case reflect.Invalid:
		return x, nil
	case reflect.String:
		return r.Resolve(x, refv.String())
	default:
		return nil, errors.New("'$ref' must be a string (got " + refv.Kind().String() + ")")
	}
}
