package jsref_test

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

func TestResolveMemory(t *testing.T) {
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

	ptrlist := make([]string, 0, len(data))
	for ptr := range data {
		ptrlist = append(ptrlist, ptr)
	}
	sort.Strings(ptrlist)

	for _, ptr := range ptrlist {
		expected := data[ptr]
		v, err := res.Resolve(m, ptr)
		if !assert.NoError(t, err, "Resolve(%s) should succeed", ptr) {
			return
		}
		if !assert.Equal(t, v, expected, "Resolve(%s) resolves to '%s'", ptr, expected) {
			return
		}
	}
}

func TestResolveFS(t *testing.T) {
	dir, err := ioutil.TempDir("", "jsref-test-")
	if !assert.NoError(t, err, "creating temporary directory should succeed") {
		return
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "obj2")
	f, err := os.Create(path)
	if !assert.NoError(t, err, "creating %s file should succeed", path) {
		return
	}
	f.Write([]byte(`{"sub":"quux"}`))
	f.Close()

	m := map[string]interface{}{
		"foo": []interface{}{
			"bar",
			map[string]interface{}{
				"$ref": "#/sub",
			},
			map[string]interface{}{
				"$ref": "file:///obj2#/sub",
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
	res.AddProvider(provider.NewFS(dir))

	ptrlist := make([]string, 0, len(data))
	for ptr := range data {
		ptrlist = append(ptrlist, ptr)
	}
	sort.Strings(ptrlist)

	for _, ptr := range ptrlist {
		expected := data[ptr]
		v, err := res.Resolve(m, ptr)
		if !assert.NoError(t, err, "Resolve(%s) should succeed", ptr) {
			return
		}
		if !assert.Equal(t, v, expected, "Resolve(%s) resolves to '%s'", ptr, expected) {
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

func TestResolveRecursive(t *testing.T) {
	var v interface{}
	src := []byte(`
{
	"foo": {
		"type": "array",
		"items": { "$ref": "#" }
	}
}`)
	if err := json.Unmarshal(src, &v); err != nil {
		log.Printf("%s", err)
		return
	}

	res := jsref.New()
	_, err := res.Resolve(v, "#/foo") // "bar"
	if !assert.NoError(t, err, "res.Resolve should succeed") {
		return
	}
}
