# go-jsref

[![Build Status](https://travis-ci.org/lestrrat/go-jsref.svg?branch=master)](https://travis-ci.org/lestrrat/go-jsref)

[![GoDoc](https://godoc.org/github.com/lestrrat/go-jsref?status.svg)](https://godoc.org/github.com/lestrrat/go-jsref)

JSON Reference Implementation for Go

# SYNOPSIS

```go
import (
  "encoding/json"
  "log"

  "github.com/lestrrat/go-jsref"
  "github.com/lestrrat/go-jsref/provider"
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
```
