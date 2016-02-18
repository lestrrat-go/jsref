package provider

import (
	"errors"
	"net/url"

	"github.com/lestrrat/go-pdebug"
)

func NewMap() *Map {
	return &Map{
		mapping: make(map[string]interface{}),
	}
}

func (mp *Map) Set(key string, v interface{}) error {
	mp.lock.Lock()
	defer mp.lock.Unlock()

	mp.mapping[key] = v
	return nil
}

func (mp *Map) Get(key *url.URL) (interface{}, error) {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START Map.Get(%s)", key)
		defer g.IRelease("END Map.Get(%s)", key)
	}

	mp.lock.Lock()
	defer mp.lock.Unlock()

	cpy := url.URL{}
	// Copy everything except for the Fragment
	cpy.Scheme = key.Scheme
	cpy.Opaque = key.Opaque
	cpy.User = key.User
	cpy.Host = key.Host
	cpy.Path = key.Path
	cpy.RawPath = key.RawPath
	cpy.RawQuery = key.RawQuery

	v, ok := mp.mapping[cpy.String()]
	if !ok {
		return nil, errors.New("not found")
	}

	return v, nil
}
