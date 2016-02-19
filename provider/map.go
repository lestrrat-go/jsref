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

func (mp *Map) Get(key *url.URL) (res interface{}, err error) {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START Map.Get(%s)", key)
		defer func() {
			if err != nil {
				g.IRelease("END Map.Get(%s): %s", key, err)
			} else {
				g.IRelease("END Map.Get(%s)", key)
			}
		}()
	}

	mp.lock.Lock()
	defer mp.lock.Unlock()

	v, ok := mp.mapping[key.String()]
	if !ok {
		return nil, errors.New("not found")
	}

	return v, nil
}
