jsref implements JSON References for Go. If there are inconsistencies, please ask
before you start your implementation.

JSON References are piece of text that can look like the following:

- #/path/to/data: a JSON pointer which points to a piece of data in an object.
- <system-specific path notation>#/path/to/data: same as above, but the object is stored in the file
- http(s)://.......#/path/to/data: same as above, but the object is stored in the remote HTTP URL.
- other values are treated as relative path to the base URI. See "Path Resolution"

An external reference should start with a file|URI component, followed
by a local reference -- i.e. start with "#", then specify a JSON pointer.

External references can be either JSON objects or YAML objects as target resources.
For YAML processing, you should use github.com/goccy/go-yaml.

It uses github.com/lestrrat-go/jsptr to evaluate the JSON pointer and drill into a specific location in the structure.

References are resolved using Resolver objects that follow this interface:

type Resolver interface {
  CanResolve(resource any) bool
  Resolve(ctx context.Context, dst any, resource any, localRef string) error
}

That is, a Resolver takes resource (which can be an object, a file path, or
an HTTP URL) and resolves the reference, a JSON pointer (prefixed with a "#")
that points to a piece of data within that resource. After evaluating the pointer against the object, it
stores the result into the variable pointed to by dst. Use github.com/lestrrat-go/blackmagic.AssignIfCompatible
to store results.

CanResolve is used to check if a resource can be handled by the Resolver.
For example, given a string, an objectResolver probably should fail.
An HTTP resolver probably should fail if the resource isn't a string, or
if it doesn't start with a https?://. A FS resolver should fail if the
path in question isn't covered in the root directory it was created with.

# StackedResolver - The main entry point.

The StackedResolver is capable of stacking resolvers, and is the object
created by the most intuitive constructor, jsref.New()

r := jsref.New() // creates jsref.StackedResolver

Multiple resolvers can be added to the StackedResolver.

r.AddResolver(r) // add other resolvers

When Resolve() is called on the StackedResolver, the added resolvers
are tried in order until one of them succeeds.

r.Resolve(dst, object, "#/path/to/data")
r.Resolve(dst, "/tmp/object.json", "#/path/to/data")
r.Resolve(dst, "https://example.com.invalid/object.json", "#/path/to/data")

By itself, it acts like the objectResolver described later. This is because
the objectResolver is ALWAYS tried as the last resort.

When attempting to resolve a resource, StackedResolver should always check
if the contained resolver can handle the resource using CanResolve()

# Utilities

Before proceeding, we should create a few utilities. Resolver objects expects
a reference and a JSON pointer in its arguments. Because JSON Reference texts
are a combination of these two, we need a function split a JSON reference into
its external (HTTP URL or file path) and its JSON pointer part

external, local, err := jsref.Split("http://example.com.invalid/data.yaml#/path/to/data)
// external = http://example.com.invalid/data.yaml
// local = #/path/to/data

Also, there should be a global `jsref.Resolve()` function that handles the most common
case. It uses a stock StackedResolver, and should have the same Resolve() signature
as other Resolvers. You shouldn't need any extra checks of limitations for this;
it should be a convenience function to a jsref.StackedResolver with HTTP resolver
and a FS resolver, as well as the default fallback object resolver. The FS resolver
should be rooted at whatever the system default root is.

One difference that `jsref.Resolve()` should have is that it should allow a full
reference, such as the following:

jsref.Resolve(dst, object, "https://example.com.invalid/object.json#/path/to/object") // ignores object
jsref.Resolve(dst, nil, "https://example.com.invalid/object.json#/path/to/object")

Since jsref.Resolve is a convenience wrapper, it use jsref.Split accordingly
to pass the correct values to the underlying StackedResolver (or other Resolvers),
because other Resolvers return an error if the `resource` parameter type is a
mismatch for that particular Resolver.

This difference in behavior should be clearly documented in both jsref.Resolve and
(jsref.StackedResolver).Resolve.

# Sub-Resolvers

All resolvers types after this point should not be exported. Their constructors
can be exported, but their struct definitions shouldn't be. The constructors
should just return the type Resolver instead of their concrete types.

## objectResolver

The objectResolver  resolves a pointer against the object given as the resource,
retrieving the data from within the object.

r := jsref.NewObjectResolver() // should return objectResolver{} (not a pointer) as it's stateless
var v any // or whatever the nderlying concrete type of the data is
err := r.Resolve(&v, object, ref)

`ref` should be a _local_ reference. That is, it should start with a "#"

The objectResolver should be as light and efficient as possible, because
it will be used extremely frequently. It should probably be stateless, and
probably be represented as either a function or an empty struct.

## fsResolver

fsResolver takes the root of the directory that it is allowed to process,
and can resolve file paths into objects before resolving the reference.

r := jsref.NewFSResolver("/") // allow everything
r := jsref.NewFSResolver("/tmp") // allow paths within /tmp

Internally it should use os.Root object to confine the operations to the
specified directory. Use os.OpenRoot() to create a *os.Root object.

## httpResolver

httpResolver resolves an HTTP reference into an object, then uses the
resolves the local reference against that object.

r := jsref.NewHTTPResolver()

# Path Resolution

Paths are resolved using a base URI. The base URI is determined by the current context.
For example, if a reference is embedded in a resource fetched from a URL, the refernce is treated relative to the URL. If it was loaded from a file, it is resolved relative
to the file path.

Unfortunately there is no way to really control this for the first call to jsref.Resolve(), so we use a context object.


```
ctx = jsref.WithBaseURI(context.Background(), "https://example.com.invalid/foo.json")
jsref.Resolve(ctx, dst, resource, ref)
```

# Working with objects

jsref should work with any Go struct, map, slice/array, or scalar. At least, it should try to do make it work.

## Map

Basic maps are easy. You just use the key names. `/foo/bar` on the following map should point to the value "here".

```
map[string]any{
  "foo": map[string]any{
    "bar": "here"
  }
}
```

However, we should allow maps with value types other than `any` as well (obviously, some references won't work, but it should do its best, and return an error otherwise). keys must be `string`, and it should return an error otherwise.

## Slices/Arrays

arrays/slices work much the same way. []any, []int, []map[string]any, etc. Give a reference like "/0", "/1", the resolver should do its best to follow the reference into indices 0 and 1 respectively. When it's just not possible, then return an error.

## Objects

we should also work with objects. First special case is when objects implement the following interface:

```
type SelfResolver interface {
  Resolve(string) (any, error)
}
```

Objects that implement this interface receive a reference fragment like `foo` and, it should return a corresponding value. For example, the previous map example may look like:

```
type MyObject struct {
  foo MyObject
  bar string
}

func (o *MyObject) Resolve(ref string) (any, error) {
  switch ref {
  case `foo`:
    return o.foo, nil
  case `bar`:
    return o.bar, nil
  default:
    return nil, fmt.Errorf(`can't resolve this`)
  }
}

MyObject{
  foo: MyObject{
    bar: "hello",
  },
}
```

If the interface is not implemented, we should use reflection to match the reference as best as possible. Only exported fields should be considered, as unexported fields will not be reachable from our resolvers. First it should try json tags. if it doesn't work, it should try the field name verbatim (of course, it would be capitalized, but that's the user's perogative)

## Circular references

Not sure about this one yet. We should create tests first, and then think about how to tackle it.
