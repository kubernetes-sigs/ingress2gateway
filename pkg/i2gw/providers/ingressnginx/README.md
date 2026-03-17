# Ingress Nginx Provider

The project supports translating ingress-nginx specific annotations. Some annotations may be translated into
implementation-specific resources or user-facing notifications depending on the selected implementation.

## Ingress Class Name

To specify the name of the Ingress class to select, use `--ingress-nginx-ingress-class=ingress-nginx` (default to 'nginx').

## IR Model

Ingress annotations are parsed into structured ingress-nginx policy fields on provider HTTPRoute contexts.
Those fields are then converted into emitter IR types defined in `pkg/i2gw/emitter_intermediate/intermediate_representation.go`,
which are consumed by implementation emitters such as `kgateway` and `agentgateway`.

## Supported Annotations

The ingress-nginx provider currently supports translating the following annotations to Gateway API and/or Kgateway-specific resources.

### Canary / Traffic Shaping

- `nginx.ingress.kubernetes.io/canary`: If set to `true`, enables weighted backends.

- `nginx.ingress.kubernetes.io/canary-by-header`: Specifies the header name used to generate an HTTPHeaderMatch.

- `nginx.ingress.kubernetes.io/canary-by-header-value`: Specifies the exact header value to match.

- `nginx.ingress.kubernetes.io/canary-by-header-pattern`: Specifies a regex pattern used in the header match.

- `nginx.ingress.kubernetes.io/canary-weight`: Specifies the backend weight for traffic shifting.

- `nginx.ingress.kubernetes.io/canary-weight-total`: Defines the total weight used when calculating backend percentages.

---

### Request / Body Size

- `nginx.ingress.kubernetes.io/client-body-buffer-size`: Sets the maximum request body size when `proxy-body-size` is not present. For the Kgateway implementation, this maps to `TrafficPolicy.spec.buffer.maxRequestSize`.

- **Note (regex-mode constraint):** Ingress NGINX session cookie paths do not support regex. If regex-mode is enabled for a host (via `use-regex: "true"` or
  `rewrite-target`) and cookie affinity is used, `session-cookie-path` must be set; the provider validates this and emits an error if it is missing.

- `nginx.ingress.kubernetes.io/proxy-body-size`: Sets the maximum allowed request body size. Takes precedence over `client-body-buffer-size`. For the Kgateway implementation, this maps to `TrafficPolicy.spec.buffer.maxRequestSize`.

---

### CORS

- `nginx.ingress.kubernetes.io/enable-cors`: Enables CORS policy generation. When set to "true", enables CORS handling for the Ingress.
  Maps to creation of a TrafficPolicy with `spec.cors` populated.
- `nginx.ingress.kubernetes.io/cors-allow-origin`: Comma-separated list of origins (e.g. "https://example.com, https://another.com").
  For the Kgateway implementation, this maps to `TrafficPolicy.spec.cors.allowOrigins`.
- `nginx.ingress.kubernetes.io/cors-allow-credentials`: Controls whether credentials are allowed in cross-origin requests ("true" / "false").
  For the Kgateway implementation, this maps to `TrafficPolicy.spec.cors.allowCredentials`.
- `nginx.ingress.kubernetes.io/cors-allow-headers`: A comma-separated list of allowed request headers. For the Kgateway implementation,
  this maps to `TrafficPolicy.spec.cors.allowHeaders`.
- `nginx.ingress.kubernetes.io/cors-expose-headers`: A comma-separated list of HTTP response headers that can be exposed to client-side
  scripts in response to a cross-origin request. For the Kgateway implementation, this maps to `TrafficPolicy.spec.cors.exposeHeaders`.
- `nginx.ingress.kubernetes.io/cors-allow-methods`: A comma-separated list of allowed HTTP methods (e.g. "GET, POST, OPTIONS").
  For the Kgateway implementation, this maps to `TrafficPolicy.spec.cors.allowMethods`.
- `nginx.ingress.kubernetes.io/cors-max-age`: Controls how long preflight responses may be cached (in seconds). For the Kgateway
  implementation, this maps to `TrafficPolicy.spec.cors.maxAge`.

### Rate Limiting

- `nginx.ingress.kubernetes.io/limit-rps`: Requests per second limit.  For the Kgateway implementation, this maps to `TrafficPolicy.spec.rateLimit.local.tokenBucket`.

- `nginx.ingress.kubernetes.io/limit-rpm`: Requests per minute limit. For the Kgateway implementation, this maps to `TrafficPolicy.spec.rateLimit.local.tokenBucket`.

- `nginx.ingress.kubernetes.io/limit-burst-multiplier`: Burst multiplier for rate limiting. Used to compute `maxTokens`.

---

### Timeouts

- `nginx.ingress.kubernetes.io/proxy-send-timeout`: Controls the request timeout. For the Kgateway implementation, this maps to `TrafficPolicy.spec.timeouts.request`.

- `nginx.ingress.kubernetes.io/proxy-read-timeout`: Controls stream idle timeout. For the Kgateway implementation, this maps to `TrafficPolicy.spec.timeouts.streamIdle`.

---

### External Auth

- `nginx.ingress.kubernetes.io/auth-url`: Specifies the URL of an external authentication service. For the Kgateway implementation, this maps to `GatewayExtension.spec.extAuth.httpService`.
- `nginx.ingress.kubernetes.io/auth-response-headers`: Comma-separated list of headers to pass to backend once authentication request completes. For the Kgateway implementation, this maps to `GatewayExtension.spec.extAuth.httpService.authorizationResponse.headersToBackend`.

### Basic Auth

- `nginx.ingress.kubernetes.io/auth-type`: Must be set to `"basic"` to enable basic authentication. For the Kgateway implementation, this maps to `TrafficPolicy.spec.basicAuth`.
- `nginx.ingress.kubernetes.io/auth-secret`: Specifies the secret containing basic auth credentials in `namespace/name` format (or just `name` if in the same namespace). For the Kgateway implementation, this maps to `TrafficPolicy.spec.basicAuth.secretRef.name`.
- `nginx.ingress.kubernetes.io/auth-secret-type`: **Only `"auth-file"` is supported** (default). The secret must contain an htpasswd file in the key `"auth"`. Only SHA hashed passwords are supported. For the Kgateway implementation, this maps to `TrafficPolicy.spec.basicAuth.secretRef.key` set to `"auth"`.

---

### Backend Protocol

- `nginx.ingress.kubernetes.io/backend-protocol`: Indicates the L7 protocol that is used to communicate with the proxied backend.
  - This annotation expresses **upstream/backend** protocol intent only. It does **not** change the generated route kind
    from `HTTPRoute` to `GRPCRoute`, because the source object is still an HTTP Ingress and the annotation only affects
    how the proxy talks to the backend.
  - **Supported values (recorded):** `GRPC`, `GRPCS`
    - The provider records protocol intent as policy metadata (used by implementation emitters).
    - For the Kgateway implementation:
      - If `service-upstream: "true"` is also enabled for the same Service backend, the Kgateway emitter stamps `spec.static.appProtocol: grpc`
        on the generated `Backend`.
      - Otherwise, the Kgateway emitter does **not** generate Kubernetes `Service` resources. Instead, it emits an **INFO** notification with a `kubectl patch`
        command to set `spec.ports[].appProtocol` on the existing Service.
      - This annotation is treated as upstream protocol metadata and does not imply `GRPCRoute` projection.
  - **Values treated as default HTTP/1.x (no-op):** `HTTP`, `HTTPS`, `AUTO_HTTP`
  - **Unsupported values (rejected):** `FCGI` (and others)
  - **Safety note:** The provider does not attempt to create or mutate Kubernetes Services; implementation emitters decide how to safely project this intent.

---

### Backend (Upstream) Configuration

- `nginx.ingress.kubernetes.io/proxy-connect-timeout`: Controls the upstream connection timeout. For the Kgateway implementation,
  this maps to `BackendConfigPolicy.spec.connectTimeout`.
- `nginx.ingress.kubernetes.io/load-balance`: Sets the algorithm to use for load balancing. The only supported value is `round_robin`.
  For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer`.

**Note:** For the Kgateway implementation, if multiple Ingress resources reference the same Service with different `proxy-connect-timeout` values, ingress2gateway will emit warnings because Kgateway cannot safely apply multiple conflicting `BackendConfigPolicy` resources to the same Service.

---

### Service Upstream

- `nginx.ingress.kubernetes.io/service-upstream`: When set to `"true"`, configures the provider to treat a Service as a single
  upstream (Service IP/port semantics) rather than per-Endpoint Pod IPs.
  - The provider **does not** directly mutate the generated `HTTPRoute`.
  - Instead, it records a provider-specific policy with:
    - derived static Backends (one per covered Service backendRef), and
    - `(rule, backend)` index pairs indicating which `HTTPRoute` backendRefs the policy applies to.
  - An implementation-specific emitter (e.g., Kgateway or Agentgateway) can then use this policy to:
    1. Emit implementation-specific `Backend` CRs, and
    2. Rewrite affected `HTTPRoute.spec.rules[].backendRefs[]` entries to reference those emitted Backend CRs.
  - Backend host is derived as in-cluster DNS (`<service>.<namespace>.svc.cluster.local`) and the Backend name is derived as
    `<service>-service-upstream`.
  - Only applies to core Service `backendRefs` (empty group and kind `Service` / unset kind).
  - Requires an explicit backendRef port (if port cannot be determined, the backendRef is skipped).

---

### Backend TLS

- `nginx.ingress.kubernetes.io/proxy-ssl-secret`: Specifies a Secret containing client certificate (`tls.crt`), client key (`tls.key`), and optionally CA certificate (`ca.crt`) in PEM format. The secret name can be specified as `secretName` (same namespace) or `namespace/secretName`. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.tls.secretRef`. **Note:** The secret must be in the same namespace as the BackendConfigPolicy.

- `nginx.ingress.kubernetes.io/proxy-ssl-verify`: Enables or disables verification of the proxied HTTPS server certificate. Values: `"on"` or `"off"` (default: `"off"`). For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.tls.insecureSkipVerify` (inverted: `"on"` = `false`, `"off"` = `true`).

- `nginx.ingress.kubernetes.io/proxy-ssl-name`: Overrides the server name used to verify the certificate of the proxied HTTPS server. This value is also passed through SNI (Server Name Indication) when establishing a connection. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.tls.sni`. Setting this value automatically enables SNI.

- `nginx.ingress.kubernetes.io/proxy-ssl-server-name`: **Note:** This annotation is not handled separately. In Kgateway, SNI is automatically enabled when `proxy-ssl-name` is set.

**Note:** For the Kgateway implementation, backend TLS configuration is applied via `BackendConfigPolicy` resources. If multiple Ingress resources reference the same Service with different backend TLS settings, ingress2gateway will create a single `BackendConfigPolicy` per Service, and conflicting settings may result in warnings.

---

### Access Logging

- `nginx.ingress.kubernetes.io/enable-access-log`: Enables or disables access logging.
  - In ingress-nginx, access logging is enabled by default when the annotation is not present.
  - When the annotation is present, the provider records an explicit boolean:
    - `"true"` enables access logging
    - any other value is treated as `false`
  - For the Kgateway implementation, when access logging is enabled, the Kgateway emitter will create an `HTTPListenerPolicy` that configures a basic Envoy access log policy via `HTTPListenerPolicy.spec.accessLog[].fileSink`. This can be further customized as needed; see the Kgateway access logging docs.

---

### Session Affinity

- `nginx.ingress.kubernetes.io/affinity`: Enables and sets the affinity type in all Upstreams of an Ingress. The only affinity type available for NGINX is "cookie". For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies` with cookie-based hash policy.

- `nginx.ingress.kubernetes.io/session-cookie-name`: Defines the name of the cookie used for session affinity. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.name`.

- `nginx.ingress.kubernetes.io/session-cookie-path`: Defines the path that will be set on the cookie. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.path`.

- `nginx.ingress.kubernetes.io/session-cookie-domain`: Sets the Domain attribute of the sticky cookie. **Note:** This annotation is parsed but not currently mapped to kgateway as the Cookie type doesn't support domain.

- `nginx.ingress.kubernetes.io/session-cookie-samesite`: Applies a SameSite attribute to the sticky cookie. Browser accepted values are None, Lax, and Strict. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.sameSite`.

- `nginx.ingress.kubernetes.io/session-cookie-expires`: Sets the TTL/expiration time for the cookie. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.ttl`.

- `nginx.ingress.kubernetes.io/session-cookie-max-age`: Sets the TTL/expiration time for the cookie. Takes precedence over `session-cookie-expires` if both are specified. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.ttl`.

- `nginx.ingress.kubernetes.io/session-cookie-secure`: Sets the Secure flag on the cookie. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.secure`.

---

### SSL Redirect

- `nginx.ingress.kubernetes.io/ssl-redirect`: When set to `"true"`, enables SSL redirect for HTTP requests. For the Kgateway implementation, this maps to a `RequestRedirect` filter on HTTPRoute rules that redirects HTTP to HTTPS with a 301 status code. Note that ingress-nginx redirects with code 308, but that isn't supported by gateway API. 

- `nginx.ingress.kubernetes.io/force-ssl-redirect`: When set to `"true"`, enables SSL redirect for HTTP requests. This annotation is treated exactly the same as `ssl-redirect`. For the Kgateway implementation, this maps to a `RequestRedirect` filter on HTTPRoute rules that redirects HTTP to HTTPS with a 301 status code. Note that ingress-nginx redirects with code 308, but that isn't supported by gateway API. 

**Note:** Both annotations are supported and treated identically. If either annotation is set to `"true"` (case-insensitive), SSL redirect will be enabled. The redirect filter is added at the rule level in the HTTPRoute, redirecting all HTTP traffic to HTTPS.

---

### SSL Passthrough (TLS Passthrough)

- `nginx.ingress.kubernetes.io/ssl-passthrough`: When set to `"true"` (case-insensitive), enables TLS passthrough.
  When enabled, TLS termination happens at the backend service rather than at the ingress controller, so the provider converts
  the affected `HTTPRoute` into a `TLSRoute` and configures a TLS passthrough `Gateway` listener.

Provider behavior:

- Converts the generated `HTTPRoute` for the affected host/group into a `TLSRoute` (same name/namespace), preserving:
  - `spec.parentRefs` (copied from the HTTPRoute)
  - `spec.hostnames` (if present)
- Rewrites per-rule backends:
  - Each HTTPRoute rule becomes a TLSRoute rule with `backendRefs`
  - BackendRef `port` is copied when present; otherwise it defaults to `443`
  - BackendRef `weight` and `namespace` are preserved when set
- Removes the original `HTTPRoute` from the IR and adds the new `TLSRoute`.

Gateway listener behavior:

- Adds a TLS passthrough listener (`protocol: TLS`, `tls.mode: Passthrough`) to the referenced parent `Gateway`.
- Listener naming/port:
  - Default: `name: tls-passthrough`, `port: 8443`
  - If a hostname is present: `name: <hostname>-tls-passthrough`, `port: 443`, and `hostname:` is set on the listener
- Removes any generated HTTP listener on the Gateway that matches the TLSRoute hostname (so only the passthrough TLS listener remains for that hostname).
- Updates the `TLSRoute.spec.parentRefs[].sectionName` to bind the route to the created passthrough listener.

**Note:** With TLS passthrough enabled, backends must be prepared to accept and terminate TLS themselves.

---

### Regex Path Matching and Rewrites

- `nginx.ingress.kubernetes.io/use-regex`: When set to `"true"`, indicates that the paths defined on that Ingress should be treated as regular expressions.
  Uses host-group semantics: if any Ingress contributing rules for a given host has `use-regex: "true"`, regex-style path matching is enforced on **all**
  paths for that host (across all contributing Ingresses).

- `nginx.ingress.kubernetes.io/rewrite-target`: Rewrites the request path using regex rewrite semantics.
  Uses host-group semantics: if any Ingress contributing rules for a given host sets `rewrite-target`, regex-style path matching is enforced on **all**
  paths for that host (across all contributing Ingresses), consistent with ingress-nginx behavior.

For the Kgateway implementation:

- When regex-mode is enabled for a host (via `use-regex: "true"` or `rewrite-target`), the emitter converts `PathPrefix` / `Exact` matches under that host
  to `RegularExpression` matches.
- For Ingresses that set `use-regex: "true"`, their contributed path strings are treated as **regex** (not escaped as literals).
- For other Ingresses under the same host (that did not set `use-regex: "true"`), their contributed path strings are treated as **literals** within a regex
  match (escaped), to preserve the original non-regex intent.
- `rewrite-target` generates `TrafficPolicy` URL rewrites using `spec.urlRewrite.pathRegex` and is attached via `ExtensionRef` filters (partial coverage).

---

## Provider Limitations

- Currently, kgateway is the only supported implementation-specific emitter.
- Some NGINX behaviors cannot be reproduced exactly due to differences between NGINX and semantics of other proxy implementations.
- Regex-mode is implemented by converting HTTPRoute path matches to `RegularExpression`. Some ingress-nginx details (such as case-insensitive `~*` behavior)
  may not be reproduced exactly depending on the underlying Gateway API / Envoy behavior and the patterns provided.

If you rely on annotations not listed above, please open an issue or be prepared to apply post-migration manual adjustments.
