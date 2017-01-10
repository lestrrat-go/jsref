package jsref_test

import (
	"encoding/json"
	"fmt"
	"log"

	jsref "github.com/lestrrat/go-jsref"
	"github.com/lestrrat/go-jsref/provider"
)

func Example() {
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

	ptrs := []string{
		"#/foo/0", // "bar"
		"#/foo/1", // "baz"
		"#/foo/2", // "quux" (resolves via `mp`)
		"#/foo",   // contents of foo key
	}
	for _, ptr := range ptrs {
		result, err := res.Resolve(v, ptr)
		if err != nil { // failed to resolve
			fmt.Printf("err: %s\n", err)
			continue
		}
		b, _ := json.Marshal(result)
		fmt.Printf("%s -> %s\n", ptr, string(b))
	}

	// OUTPUT:
	// #/foo/0 -> "bar"
	// #/foo/1 -> "baz"
	// #/foo/2 -> "quux"
	// #/foo -> ["bar",{"$ref":"#/sub"},{"$ref":"obj2#/sub"}]
}
