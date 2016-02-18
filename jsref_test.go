package jsref_test

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lestrrat/go-jsref"
	"github.com/lestrrat/go-jsref/provider"
	"github.com/stretchr/testify/assert"
)

func Example() {
	var v interface{}
	src := []byte(`
{
  "foo": ["bar", {"$ref": "#/sub"}, {"$ref", "obj2#/sub"}],
  "sub": "baz"
}`)
	if err := json.Unmarshal(src, &v); err != nil {
		log.Printf("%s", err)
		return
	}

	// External reference
	mp := provider.NewMap()
	mp.Set("obj2", map[string]string{"sub": "quux"})

	res := jsref.New()
	res.AddProvider(mp) // Register the provider

	res.Resolve(v, "#/foo/0") // "bar"
	res.Resolve(v, "#/foo/1") // "baz"
	res.Resolve(v, "#/foo/2") // "quux" (resolve via `mp`)
}

func TestResolve(t *testing.T) {
	m := map[string]interface{}{
		"foo": []interface{}{
			"bar",
			map[string]interface{}{
				"$ref": "#/sub",
			},
			map[string]interface{}{
				"$ref": "obj2#/sub",
			},
		},
		"sub": "baz",
	}

	data := map[string]string{
		"#/foo/0": "bar",
		"#/foo/1": "baz",
		"#/foo/2": "quux",
	}

	res := jsref.New()
	mp := provider.NewMap()
	mp.Set("obj2", map[string]string{"sub": "quux"})
	res.AddProvider(mp)
	for ptr, expected := range data {
		v, err := res.Resolve(m, ptr)
		if !assert.NoError(t, err, "Resolve(%s) should succeed", ptr) {
			return
		}
		if !assert.Equal(t, v, expected, "Resolve(%s) resolves to '%s'", expected) {
			return
		}
	}
}

func TestResolveHTTP(t *testing.T) {
	if b, _ := strconv.ParseBool(os.Getenv("JSREF_LIVE_TESTS")); !b {
		t.Skip("JSREF_LIVE_TESTS is not available, skipping test")
	}

	cl := http.Client{
		Transport: &http.Transport{
			Dial: func(n, a string) (net.Conn, error) {
				return net.DialTimeout(n, a, 2*time.Second)
			},
		},
	}

	const schemaURL = `http://json-schema.org/draft-04/schema#`
	if _, err := cl.Get(schemaURL); err != nil {
		t.Skip("JSON schema '" + schemaURL + "' unavailable, skipping test")
	}

	res := jsref.New()
	hp := provider.NewHTTP()
	res.AddProvider(hp)

	m := map[string]interface{}{
		"fetch": map[string]string{
			"$ref": schemaURL,
		},
	}

	ptr := "#/fetch"
	v, err := res.Resolve(m, ptr)
	if !assert.NoError(t, err, "Resolve(%s) should succeed", ptr) {
		return
	}

	switch v.(type) {
	case map[string]interface{}:
		mv := v.(map[string]interface{})
		if !assert.Equal(t, mv["id"], schemaURL, "Resolve("+schemaURL+") resolved to JSON schema") {
			return
		}
	default:
		t.Errorf("Expected map[string]interface{}")
	}
}