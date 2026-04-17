# Ingress Nginx Provider

The project supports translating [ingress-nginx](https://kubernetes.github.io/ingress-nginx/) specific annotations into Gateway API resources.

**Ingress class name**

To specify the name of the Ingress class to select, use `--ingress-nginx-ingress-class=ingress-nginx` (defaults to `nginx`).

## Supported Annotations

### Canary

- `nginx.ingress.kubernetes.io/canary`: If set to `true`, enables canary routing for this Ingress.
- `nginx.ingress.kubernetes.io/canary-by-header`: The header name used to generate an `HTTPHeaderMatch` for the routes from this Ingress. If not specified, no `HTTPHeaderMatch` is generated.
- `nginx.ingress.kubernetes.io/canary-by-header-value`: The header value to perform an exact match on in the generated `HTTPHeaderMatch`.
- `nginx.ingress.kubernetes.io/canary-weight`: If specified and non-zero, this value is applied as the weight of the backends for routes generated from this Ingress.
- `nginx.ingress.kubernetes.io/canary-weight-total`: The total weight to use when calculating canary traffic split (defaults to 100).
- `nginx.ingress.kubernetes.io/canary-by-header-pattern`: **Recognized but not converted.** A warning is emitted; header pattern matching is not supported in the conversion.
- `nginx.ingress.kubernetes.io/canary-by-cookie`: **Recognized but not converted.** A warning is emitted; cookie-based canary routing is not supported in the conversion.

### Rewrite

- `nginx.ingress.kubernetes.io/rewrite-target`: Converts to a Gateway API `URLRewrite` filter with `ReplaceFullPath`. Note: path rewrites with capture group references (e.g. `$1`) are not supported and will be flagged.
- `nginx.ingress.kubernetes.io/app-root`: Converts to an `HTTPRequestRedirect` filter (status code 302) with `ReplaceFullPath` set to the annotation value. Adds or updates a rule with an exact `"/"` path match to redirect requests to the specified path.

### Redirect

- `nginx.ingress.kubernetes.io/permanent-redirect`: Converts to an `HTTPRequestRedirect` filter (default status code 301).
- `nginx.ingress.kubernetes.io/permanent-redirect-code`: Overrides the status code for permanent redirects (supported: 301, 302, 303, 307, 308).
- `nginx.ingress.kubernetes.io/temporal-redirect`: Converts to an `HTTPRequestRedirect` filter (default status code 302). Takes priority over permanent redirect.
- `nginx.ingress.kubernetes.io/temporal-redirect-code`: Overrides the status code for temporal redirects (supported: 301, 302, 303, 307).
- `nginx.ingress.kubernetes.io/ssl-redirect`: When set to `false`, disables the automatic HTTP-to-HTTPS redirect (308) that is otherwise added for TLS-configured Ingresses.
- `nginx.ingress.kubernetes.io/proxy-redirect-from`: **Recognized but not converted.** A warning is emitted.
- `nginx.ingress.kubernetes.io/proxy-redirect-to`: **Recognized but not converted.** A warning is emitted.

### Headers

- `nginx.ingress.kubernetes.io/upstream-vhost`: Sets the `Host` request header via an `HTTPRequestHeaderModifier` filter.
- `nginx.ingress.kubernetes.io/connection-proxy-header`: Sets the `Connection` request header via an `HTTPRequestHeaderModifier` filter.
- `nginx.ingress.kubernetes.io/x-forwarded-prefix`: When used alongside `rewrite-target`, adds the `X-Forwarded-Prefix` request header.
- `nginx.ingress.kubernetes.io/custom-headers`: **Recognized but not converted.** A warning is emitted.

### Timeouts

- `nginx.ingress.kubernetes.io/proxy-connect-timeout`: Converted to Gateway API timeout configuration (value is in seconds).
- `nginx.ingress.kubernetes.io/proxy-send-timeout`: Converted to Gateway API timeout configuration (value is in seconds).
- `nginx.ingress.kubernetes.io/proxy-read-timeout`: Converted to Gateway API timeout configuration (value is in seconds).

### Body Size

- `nginx.ingress.kubernetes.io/proxy-body-size`: Maximum request body size. Converted from nginx size format (e.g. `10m`) to Kubernetes resource quantity.
- `nginx.ingress.kubernetes.io/client-body-buffer-size`: Client body buffer size. Converted from nginx size format to Kubernetes resource quantity.

### Backend Protocol

- `nginx.ingress.kubernetes.io/backend-protocol`: Routes with `GRPC` or `GRPCS` are converted to `GRPCRoute` resources. Routes with `HTTPS` or `GRPCS` trigger `BackendTLSPolicy` creation. Unsupported values (e.g. `FCGI`) emit a warning.

### Regex

- `nginx.ingress.kubernetes.io/use-regex`: When set to `true`, path match types are converted to `PathMatchRegularExpression`.

### CORS

- `nginx.ingress.kubernetes.io/enable-cors`: Enables CORS configuration for the route.
- `nginx.ingress.kubernetes.io/cors-allow-origin`: Allowed origins (comma-separated, defaults to `*`).
- `nginx.ingress.kubernetes.io/cors-allow-methods`: Allowed HTTP methods (comma-separated, defaults to `GET, PUT, POST, DELETE, PATCH, OPTIONS`).
- `nginx.ingress.kubernetes.io/cors-allow-headers`: Allowed request headers (comma-separated).
- `nginx.ingress.kubernetes.io/cors-expose-headers`: Headers exposed to the browser (comma-separated).
- `nginx.ingress.kubernetes.io/cors-allow-credentials`: Whether credentials are allowed (defaults to `true`).
- `nginx.ingress.kubernetes.io/cors-max-age`: Max age in seconds for preflight cache (defaults to `1728000`).

### IP Range Control

- `nginx.ingress.kubernetes.io/whitelist-source-range`: Comma-separated list of allowed source CIDRs.
- `nginx.ingress.kubernetes.io/denylist-source-range`: Comma-separated list of denied source CIDRs.

### Backend TLS

- `nginx.ingress.kubernetes.io/proxy-ssl-verify`: Must be set to `on` for `BackendTLSPolicy` creation.
- `nginx.ingress.kubernetes.io/proxy-ssl-secret`: The Kubernetes Secret containing the trusted CA certificate (required for `BackendTLSPolicy`).
- `nginx.ingress.kubernetes.io/proxy-ssl-name`: The hostname for TLS server name validation (required for `BackendTLSPolicy`).
- `nginx.ingress.kubernetes.io/proxy-ssl-server-name`: Must be set to `on` to enable SNI (required for `BackendTLSPolicy`).
- `nginx.ingress.kubernetes.io/proxy-ssl-verify-depth`: **Recognized but not converted.** TLS verification depth is not supported by Gateway API.
- `nginx.ingress.kubernetes.io/proxy-ssl-protocols`: **Recognized but not converted.** TLS protocol configuration is not supported by Gateway API.

### SSL Passthrough

- `nginx.ingress.kubernetes.io/ssl-passthrough`: When set to `true`, the HTTPRoute for the Ingress is replaced by a `TLSRoute` with TLS passthrough mode. A new TLS listener (port 443, `Passthrough` mode) is added to the Gateway for each matching hostname.

If you are reliant on any annotations not listed above, please open an issue. In the meantime you'll need to manually find a Gateway API equivalent.
