package provider

import (
	"errors"
	"net/url"
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
	mp.lock.Lock()
	defer mp.lock.Unlock()

	// All components except .Path is ignored
	v, ok := mp.mapping[key.Path]
	if !ok {
		return nil, errors.New("not found")
	}

	return v, nil
}
