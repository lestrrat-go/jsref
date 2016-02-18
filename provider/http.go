package provider

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

func NewHTTP() *HTTP {
	return &HTTP{
		mp: NewMap(),
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (hp *HTTP) Get(key *url.URL) (interface{}, error) {
	v, err := hp.mp.Get(key)
	if err == nil { // Found!
		return v, nil
	}

	res, err := hp.Client.Get(key.String())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	dec := json.NewDecoder(res.Body)

	var x interface{}
	if err := dec.Decode(&x); err != nil {
		return nil, err
	}

	return x, nil
}
