# OpenAPI Provider

The provider translates OpenAPI Specification (OAS) 3.x documents to Kubernetes Gateway API resources Gateway and HTTPRoute.

## Example

```sh
./ingress2gateway print --providers openapi --input-file=petstore3-openapi.json
```

## Known limitations

* Only offline translation supported – i.e. `--input-file` required
* All API operation [paths](https://swagger.io/specification/v3/#paths-object) treated as `Exact` type – i.e. no support for [path templating](https://swagger.io/specification/v3/#path-templating), therefore no `PathPrefix`, nor `RegularExpression` path types output
* Limited support for [parameters](https://swagger.io/specification/v3/#parameter-object) – only required `header` and `query` parameters supported
* Limited support to [server variables](https://swagger.io/specification/v3/#server-variable-object) – only limited sets (`enum`) supported
* No support to [references](https://swagger.io/specification/v3/#reference-object) (`$ref`)
* No support to [external documents](https://swagger.io/specification/v3/#external-documentation-object)

Additionally, no support to any OpenAPI feature with no direct equivalent to core Gateway API fields, such as [request bodies](https://swagger.io/specification/v3/#request-body-object), [examples](https://swagger.io/specification/v3/#example-object), [security schemes](https://swagger.io/specification/v3/#security-scheme-object), [callbacks](https://swagger.io/specification/v3/#callback-object), [extensions](https://swagger.io/specification/v3/#specification-extensions), etc.
