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
func (r *Resolver) Resolve(v interface{}, spec string) (interface{}, error) {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START Resolver.Resolve(%s)", spec)
		defer g.IRelease("END Resolver.Resolve(%s)", spec)
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

	var x interface{}
	ptr := u.Fragment
	if ptr == "" {
		x = v
	} else {
		if pdebug.Enabled {
			pdebug.Printf("Using JSON Pointer '%s'", ptr)
		}
		p, err := jspointer.New(ptr)
		if err != nil {
			return nil, err
		}

		x, err = p.Get(v)
		if err != nil {
			return nil, err
		}
	}

	rv := reflect.ValueOf(x)
	if rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map:
		refv := rv.MapIndex(reflect.ValueOf("$ref"))
		if refv.Kind() == reflect.Interface {
			refv = refv.Elem()
		}
		switch refv.Kind() {
		case reflect.Invalid:
			// no op
		case reflect.String:
			return r.Resolve(v, refv.String())
		default:
			return nil, errors.New("'$ref' must be a string (got " + refv.Kind().String() + ")")
		}
	case reflect.Struct:
		i := structinfo.StructFieldFromJSONName(rv, "$ref")
		refv := rv.Field(i)
		if refv.Kind() == reflect.Interface {
			refv = refv.Elem()
		}
		switch refv.Kind() {
		case reflect.Invalid:
			// no op
		case reflect.String:
			return r.Resolve(v, refv.String())
		default:
			return nil, errors.New("'$ref' must be a string (got " + refv.Kind().String() + ")")
		}
	}

	return x, nil
}
