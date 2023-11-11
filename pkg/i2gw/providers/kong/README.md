# Ingress Kong Provider

## Annotations supported

The project supports translating Kong-specific annotations.

Current supported annotations:

- `konghq.com/methods`: If specified, the values of this annotation are used to
  perform method matching on the associated ingress rules. Multiple methods can
  be specified by separating values with commas. Example: `konghq.com/methods: "POST,GET"`.
- `konghq.com/headers.*`: If specified, the values of this annotation are used to
  perform header matching on the associated ingress rules. The header name is specified
  in the annotation key after `.`, and the annotations value can contain multiple
  header values separated by commas. All the header values for a specific header
  name are intended to be ORed. Example: `konghq.com/headers.x-routing: "alpha,bravo"`.
- `konghq.com/plugins`: If specified, the values of this annotation are used to
  configure plugins on the associated ingress rules. Multiple plugins can be specified
  by separating values with commas. Example: `konghq.com/plugins: "plugin1,plugin2"`.

If you are reliant on any annotations not listed above, please open an issue.

## Implementation-specific features

The following implementation-specific features are supported:

- The ingress `ImplementationSpecific` match type is properly converted to
  - `RegularExpression` HTTPRoute match type when the path has the prefix `/~`.
  - `PathPrefix` HTTPRoute match type when there is no prefix `/~`.
