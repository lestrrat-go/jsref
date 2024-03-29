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
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jsref"
	"github.com/lestrrat-go/jsref/provider"
	"github.com/stretchr/testify/assert"
)

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
	if !assert.NoError(t, mp.Set("obj2", map[string]string{"sub": "quux"}), `mp.Set("obj2") should succeed`) {
		return
	}

	if !assert.NoError(t, res.AddProvider(mp), `res.AddProvider() should succeed`) {
		return
	}

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

	// In this test we test if we can optionally recursively
	// resolve references
	v, err := res.Resolve(m, "#/foo", jsref.WithRecursiveResolution(true))
	if !assert.NoError(t, err, "Resolve(%s) should succeed", "#/foo") {
		return
	}

	if !assert.Equal(t, []interface{}{"bar", "baz", "quux"}, v) {
		return
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
	_, _ = f.Write([]byte(`{"sub":"quux"}`))
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
	if !assert.NoError(t, res.AddProvider(provider.NewFS(dir)), `res.AddProvider() should succeed`) {
		return
	}

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
	if !assert.NoError(t, res.AddProvider(hp), `res.AddProvider() should succeed`) {
		return
	}

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

	switch v := v.(type) {
	case map[string]interface{}:
		if !assert.Equal(t, v["id"], schemaURL, "Resolve("+schemaURL+") resolved to JSON schema") {
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
		"items": [{ "$ref": "#" }]
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

func TestGHPR12(t *testing.T) {
	// https://github.com/lestrrat-go/jsref/pull/2 gave me an example
	// using "foo" as the JS pointer (could've been a typo)
	// but it gave me weird results, so this is where I'm testing it
	var v interface{}
	src := []byte(`
{
	"foo": "bar"
}`)
	if err := json.Unmarshal(src, &v); err != nil {
		log.Printf("%s", err)
		return
	}

	res := jsref.New()
	_, err := res.Resolve(v, "foo")
	if !assert.NoError(t, err, "res.Resolve should fail") {
		return
	}
}

func TestHyperSchemaRecursive(t *testing.T) {
	src := []byte(`
{
  "definitions": {
    "virtual_machine": {
      "type": "object"
    }
  },
  "links": [
    {
      "schema": {
        "type": "object"
      },
      "targetSchema": {
        "$ref": "#/definitions/virtual_machine"
      }
    },
    {
      "targetSchema": {
        "type": "array",
        "items": {
          "$ref": "#/definitions/virtual_machine"
        }
      }
    }
  ]
}`)
	var v interface{}
	err := json.Unmarshal(src, &v)
	assert.Nil(t, err)
	res := jsref.New()

	ptrs := []string{
		"#/links/0/schema",
		"#/links/0/targetSchema",
		"#/links/1/targetSchema",
	}
	for _, ptr := range ptrs {
		result, err := res.Resolve(v, ptr, jsref.WithRecursiveResolution(true))
		assert.Nil(t, err)
		b, err := json.Marshal(result)
		if !assert.NoError(t, err, "json.Marshal should succeed") {
			return
		}
		if !assert.False(t, strings.Contains(string(b), "$ref"), "%s did not recursively resolve", ptr) {
			t.Logf("resolved to '%s'", b)
			return
		}
	}
}

func TestGHIssue7(t *testing.T) {
	src := []byte(`{
  "status": {
    "type": ["string", "null"],
    "enum": [
      "sent",
      "duplicate",
      "error",
      "invalid",
      "rejected",
      "unqueued",
      "unsubscribed",
      null
    ]
  }
}`)

	var v interface{}
	if !assert.NoError(t, json.Unmarshal(src, &v), `Unmarshal should succeed`) {
		return
	}

	res := jsref.New()
	result, err := res.Resolve(v, "", jsref.WithRecursiveResolution(true))
	t.Logf("%s", result)
	t.Logf("%s", err)
}

func TestReferenceNewRoot(t *testing.T) {
	obj1 := map[string]interface{}{
		"foo": []interface{}{
			"bar",
			map[string]interface{}{
				"$ref": "#/sub",
			},
			map[string]interface{}{
				"$ref": "obj2#/bar/sub",
			},
			map[string]interface{}{
				"$ref": "obj2#/baz",
			},
			map[string]interface{}{
				"$ref": "obj2#/hoge",
			},
		},
		"sub": "baz",
		"piyo": map[string]interface{}{
			// piyo loops on obj1 and obj2, but should not be problem unless we dereference
			"$ref": "obj2#",
		},
	}
	obj2 := map[string]interface{}{
		"bar": map[string]interface{}{
			"sub": "quux",
		},
		"baz": map[string]interface{}{
			"$ref": "obj1#/sub",
		},
		"hoge": map[string]interface{}{
			// should refer to obj2, not obj1
			"$ref": "#/bar/sub",
		},
		"fuga": map[string]interface{}{
			"$ref": "#",
		},
		"piyo": map[string]interface{}{
			"$ref": "obj1#",
		},
	}

	data := map[string]string{
		"#/foo/0": "bar",
		"#/foo/1": "baz",
		"#/foo/2": "quux",
		"#/foo/3": "baz",
		"#/foo/4": "quux",
	}

	res := jsref.New()
	mp := provider.NewMap()
	if !assert.NoError(t, mp.Set("obj1", obj1), `mp.Set("obj1") should succeed`) {
		return
	}
	if !assert.NoError(t, mp.Set("obj2", obj2), `mp.Set("obj2") should succeed`) {
		return
	}
	if !assert.NoError(t, res.AddProvider(mp), `res.AddProvider() should succeed`) {
		return
	}

	ptrlist := make([]string, 0, len(data))
	for ptr := range data {
		ptrlist = append(ptrlist, ptr)
	}
	sort.Strings(ptrlist)

	for _, ptr := range ptrlist {
		expected := data[ptr]
		v, err := res.Resolve(obj1, ptr)
		if !assert.NoError(t, err, "Resolve(%s) should succeed", ptr) {
			return
		}
		if !assert.Equal(t, v, expected, "Resolve(%s) resolves to '%s'", ptr, expected) {
			return
		}
	}

	// In this test we test if we can optionally recursively
	// resolve references
	v, err := res.Resolve(obj1, "#/foo", jsref.WithRecursiveResolution(true))
	if !assert.NoError(t, err, "Resolve(%s) should succeed", "#/foo") {
		return
	}

	if !assert.Equal(t, []interface{}{"bar", "baz", "quux", "baz", "quux"}, v) {
		return
	}
}

func TestReferenceLoop(t *testing.T) {
	obj1 := map[string]interface{}{
		"foo": []interface{}{
			"bar",
			map[string]interface{}{
				"$ref": "#/sub",
			},
			map[string]interface{}{
				"$ref": "obj2#/bar",
			},
			map[string]interface{}{
				"$ref": "obj2#/baz",
			},
		},
		"sub": "baz",
	}
	obj2 := map[string]interface{}{
		"bar": map[string]interface{}{
			"$ref": "obj1#/sub",
		},
		"baz": map[string]interface{}{
			"$ref": "obj1#/foo",
		},
	}

	data := map[string]string{
		"#/foo/0": "bar",
		"#/foo/1": "baz",
	}

	res := jsref.New()
	mp := provider.NewMap()
	if !assert.NoError(t, mp.Set("obj1", obj1), `mp.Set("obj1") should succeed`) {
		return
	}
	if !assert.NoError(t, mp.Set("obj2", obj2), `mp.Set("obj2") should succeed`) {
		return
	}
	if !assert.NoError(t, res.AddProvider(mp), `res.AddProvider() should succeed`) {
		return
	}

	ptrlist := make([]string, 0, len(data))
	for ptr := range data {
		ptrlist = append(ptrlist, ptr)
	}
	sort.Strings(ptrlist)

	for _, ptr := range ptrlist {
		expected := data[ptr]
		v, err := res.Resolve(obj1, ptr)
		if !assert.NoError(t, err, "Resolve(%s) should succeed", ptr) {
			return
		}
		if !assert.Equal(t, v, expected, "Resolve(%s) resolves to '%s'", ptr, expected) {
			return
		}
	}

	// Recursive resolution cause infinite loop.
	// Should report an error.
	_, err := res.Resolve(obj1, "#/foo", jsref.WithRecursiveResolution(true))
	if !assert.Error(t, err, "Resolve(%s) recursive should cause ErrReferenceLoop", "#/foo") {
		return
	}

	// Non-Recursive resolution should not report an error.
	_, err = res.Resolve(obj1, "#/foo")
	if !assert.NoError(t, err, "Resolve(%s) non-recursive should succeed", "#/foo") {
		return
	}
}
