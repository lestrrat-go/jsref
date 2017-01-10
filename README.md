# go-jsref

[![Build Status](https://travis-ci.org/lestrrat/go-jsref.svg?branch=master)](https://travis-ci.org/lestrrat/go-jsref)

[![GoDoc](https://godoc.org/github.com/lestrrat/go-jsref?status.svg)](https://godoc.org/github.com/lestrrat/go-jsref)

JSON Reference Implementation for Go

# SYNOPSIS

```go
package main

import (
  "fmt"
  "encoding/json"
  "log"

  "github.com/lestrrat/go-jsref"
  "github.com/lestrrat/go-jsref/provider"
)

func main() {
  var v interface{}
  src := []byte(`
{
  "foo": ["bar", {"$ref": "#/sub"}, {"$ref": "obj2#/sub"}],
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

  ptrs := []string{"#/foo/0", "#/foo/1", "#/foo/2", "foo"}
  for _, ptr := range(ptrs) {
    result, _ := res.Resolve(v, ptr)
    b, _ := json.Marshal(result)
    fmt.Printf("%s -> %s\n", ptr, string(b))
  }
}
```

# Providers

The Resolver object by default does not know how to resolve *any* reference:
You must provide it one or more `Provider`s to look for and resolve external references.

Currently available `Provider`s are:

| Name          | Description |
|:--------------|:------------|
| provider.FS   | Resolve from local file system. References must start with a `file:///` prefix |
| provider.Map  | Resolve from in memory map. |
| provider.HTTP | Resolve by making HTTP requests. References must start with a `http(s?)://` prefix |

# References

| Name                                                     | Notes                            |
|:--------------------------------------------------------:|:---------------------------------|
| [go-jsval](https://github.com/lestrrat/go-jsval)         | Validator generator              |
| [go-jshschema](https://github.com/lestrrat/go-jsschema)  | JSON Hyper Schema implementation |
| [go-jsschema](https://github.com/lestrrat/go-jsschema)   | JSON Schema implementation       |
| [go-jspointer](https://github.com/lestrrat/go-jspointer) | JSON Pointer implementations     |
