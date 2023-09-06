# Ingress Nginx Provider

The project supports translating ingress-nginx specific annotations.

Current supported annotations:

- `nginx.ingress.kubernetes.io/canary`: If set to true will enable weighting backends.
- `nginx.ingress.kubernetes.io/canary-by-header`: If specified, the value of this annotation is the header name that will be added as a HTTPHeaderMatch for the routes
- generated from this Ingress. If not specified, no HTTPHeaderMatch will be generated.
- `nginx.ingress.kubernetes.io/canary-by-header-value`: If specified, the value of this annotation is the header value to perform an HeaderMatchExact match on in the generated HTTPHeaderMatch.
- `nginx.ingress.kubernetes.io/canary-by-header-pattern`: If specified, this is the pattern to match against for the HTTPHeaderMatch, which will be of type HeaderMatchRegularExpression.
- `nginx.ingress.kubernetes.io/canary-weight`: If specified and non-zero, this value will be applied as the weight of the backends for the routes generated from this Ingress resource.
`nginx.ingress.kubernetes.io/canary-weight-total`

If you are reliant on any annotations not listed above, please open an issue. In the meantime you'll need to manually find a Gateway API equivalent.