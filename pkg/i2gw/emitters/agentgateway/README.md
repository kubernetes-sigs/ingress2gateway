# Agentgateway Emitter

The Agentgateway Emitter supports generating **Gateway API** resources plus **agentgateway**-specific extensions
from Ingress manifests using:

- **Provider**: `ingress-nginx`

**Note:** All other providers will be ignored by the emitter.

## What it outputs

- Standard **Gateway API** objects (Gateways, HTTPRoutes, etc.)
- Agentgateway extension objects emitted as unstructured resources, e.g. `AgentgatewayPolicy`.

The emitter also ensures that any generated Gateway resources use:

- `spec.gatewayClassName: agentgateway`

## Development Workflow

The typical development workflow for adding an Ingress NGINX feature to the Agentgateway emitter is:

1. Use [this issue](https://github.com/kubernetes-sigs/ingress2gateway/issues/232) to prioritize the list of Ingress NGINX features unless
   the business provides requirements. When this list is complete, refer to [this doc](https://docs.google.com/document/d/12ejNTb45hASGYvUjd3t9mfNwuMTX3KBM68ALM4jRFBY/edit?usp=sharing) for additional features. **Note:** Several of the features from the above list have already been implemented, so review the
   current supported features before adding more.
2. If a feature cannot map to an existing agentgateway API, open an Agentgateway issue describing what’s needed.
3. Extend the ingress-nginx emitter IR/generic Policy IR (`pkg/i2gw/emitter_intermediate/intermediate_representation.go`) as needed so features are represented in a structured way.
4. Add a feature-specific function to the ingress-nginx provider (`pkg/i2gw/providers/ingressnginx`), e.g.
   `rateLimitFeature()`, that parses the Ingress NGINX annotation(s) and records them as ingress-nginx policy IR
   that is converted into emitter IR.
5. Update the Agentgateway Emitter (`pkg/i2gw/emitters/agentgateway/emitter.go`) to consume the IR and emit
   agentgateway-specific resources.
6. Add/extend integration and e2e tests to cover the new behavior.
7. Update the list of supported annotations with the feature you added.
8. Submit a PR to merge your changes upstream. [This branch](https://github.com/danehans/ingress2gateway/tree/impl_emitter_nginx_feat) is the **current** upstream, but [k8s-sigs](https://github.com/kubernetes-sigs/ingress2gateway) or [solo](https://github.com/solo-io/ingress2gateway) repos should be used before releasing.

## Testing

Run the tool with a test input manifest:

```bash
go run . print \
  --providers=ingress-nginx \
  --emitter=agentgateway \
  --input-file ./pkg/i2gw/emitters/agentgateway/testing/testdata/<FEATURE>.yaml
```

The command should generate Gateway API resources plus agentgateway extension resources (when applicable).

## Notifications

Some conversions require follow-up user action that cannot be expressed safely as emitted manifests. In those cases,
the agentgateway emitter emits **INFO** notifications on the CLI during conversion.

Currently, the agentgateway emitter emits a notification when projecting **Basic Authentication**, because:

- ingress-nginx (auth-file) commonly expects htpasswd content under the Secret key **`auth`**
- agentgateway expects htpasswd content under the Secret key **`.htaccess`**

## Supported Annotations

### Traffic Behavior

#### SSL Redirect

The agentgateway emitter supports projecting **HTTP → HTTPS redirects** by **splitting** the generated HTTPRoute into two routes when
either of the following Ingress NGINX annotations are truthy:

- `nginx.ingress.kubernetes.io/ssl-redirect`
- `nginx.ingress.kubernetes.io/force-ssl-redirect`

This is implemented using Gateway API `HTTPRoute` `RequestRedirect` filters (for the HTTP listener) and a separate HTTPS-bound
`HTTPRoute` that preserves backend routing.

**Semantics:**

- A new **HTTP redirect route** is generated:
  - Bound to the Gateway **HTTP** listener (`parentRefs[].sectionName: <hostname>-http`)
  - Each rule includes a `RequestRedirect` filter with:
    - `scheme: https`
    - `statusCode: 301`
  - **No** `backendRefs` are present (Gateway API does not allow `RequestRedirect` filters and backends in the same rule).
- A new **HTTPS backend route** is generated:
  - Bound to the Gateway **HTTPS** listener (`parentRefs[].sectionName: <hostname>-https`)
  - Preserves the original backendRefs
  - Any existing `RequestRedirect` filters are removed (if present)

**Naming:**

- HTTP redirect route: `<original-http-route-name>-http-redirect`
- HTTPS backend route: `<original-http-route-name>-https`

**Notes:**

- If an HTTPS listener cannot be determined for the hostname, the emitter will still attempt to emit the HTTP redirect route when
  an HTTP listener exists; the HTTPS backend route is omitted in that case.
- Redirect behavior is implemented purely with Gateway API objects (no agentgateway extensions are required).

#### Rewrite Target

The agentgateway emitter supports rewriting request paths via:

- `nginx.ingress.kubernetes.io/rewrite-target`

and (for regex/capture-group behavior):

- `nginx.ingress.kubernetes.io/use-regex: "true"`

This is projected into an `AgentgatewayPolicy` using agentgateway’s `Traffic.Transformation` model by setting the
HTTP pseudo-header `:path` in `spec.traffic.transformation.request.set`.

Mappings:

- **Non-regex rewrite** (default): sets `:path` to a literal string value:
  - `rewrite-target: /new` → `AgentgatewayPolicy.spec.traffic.transformation.request.set[:path] = '"/new"'`
- **Regex rewrite** (`use-regex: "true"`): rewrites the request path using a CEL `regexReplace(...)` expression:
  - `rewrite-target: /new/$1` → `AgentgatewayPolicy.spec.traffic.transformation.request.set[:path] = 'regexReplace(request.path, "<pattern>", "/new/$1")'`

**Notes:**

- Agentgateway represents transformations using **CEL expressions**. As a result, literal strings are expressed as
  quoted CEL string literals (for example `'"/authz"'` or `'"/new"'`) rather than raw strings.
- When `use-regex: "true"` is set, the emitter derives the `<pattern>` from the generated `HTTPRoute` rule path
  regular expression (so capture groups `$1`, `$2`, … behave like ingress-nginx). If the rule contains zero or
  multiple distinct regex match values, the emitter falls back to `^(.*)`.

#### CORS

The agentgateway emitter supports projecting CORS behavior based on the following Ingress NGINX annotations:

- `nginx.ingress.kubernetes.io/enable-cors`
- `nginx.ingress.kubernetes.io/cors-allow-origin`
- `nginx.ingress.kubernetes.io/cors-allow-methods`
- `nginx.ingress.kubernetes.io/cors-allow-headers`
- `nginx.ingress.kubernetes.io/cors-expose-headers`
- `nginx.ingress.kubernetes.io/cors-allow-credentials`
- `nginx.ingress.kubernetes.io/cors-max-age`

These are mapped into an `AgentgatewayPolicy` using agentgateway’s `Traffic.Cors` model (which inlines the Gateway API `HTTPCORSFilter`):

- `enable-cors`  `cors-allow-origin` → `AgentgatewayPolicy.spec.traffic.cors.allowOrigins`
- `cors-allow-headers` → `AgentgatewayPolicy.spec.traffic.cors.allowHeaders`
- `cors-expose-headers` → `AgentgatewayPolicy.spec.traffic.cors.exposeHeaders`
- `cors-allow-methods` → `AgentgatewayPolicy.spec.traffic.cors.allowMethods`
- `cors-allow-credentials` → `AgentgatewayPolicy.spec.traffic.cors.allowCredentials`
- `cors-max-age` → `AgentgatewayPolicy.spec.traffic.cors.maxAge`

**Notes:**

- The emitter only projects CORS when `enable-cors` is truthy **and** at least one value is present in `cors-allow-origin`.
- `cors-allow-origin` values are de-duped while preserving order; empty values are ignored.
- Header lists (`cors-allow-headers`, `cors-expose-headers`) are de-duped case-insensitively.
- Method values are normalized to upper-case and filtered to valid Gateway API HTTP methods (plus `*`); unknown values are ignored.
- If `cors-max-age` is unset or non-positive, it is not projected.

##### Upstream CORS header stripping

When CORS is projected, the emitter also adds a Gateway API `ResponseHeaderModifier` filter to the generated `HTTPRoute`
rules to remove common CORS response headers from the upstream/backend response:

- `Access-Control-Allow-Origin`
- `Access-Control-Allow-Methods`
- `Access-Control-Allow-Headers`
- `Access-Control-Expose-Headers`
- `Access-Control-Max-Age`
- `Access-Control-Allow-Credentials`

**Why?**

Some backends unconditionally emit permissive CORS headers (for example `Access-Control-Allow-Origin: *`), which can
cause disallowed Origins to appear “allowed” even when the gateway policy is configured with a restricted allowlist.
Stripping these upstream headers ensures that the effective CORS behavior is controlled by the emitted policy.

**Impact:**

If your application intentionally manages CORS by emitting its own CORS headers, enabling CORS on the Ingress and using
the agentgateway emitter will suppress those upstream headers. Configure the desired CORS behavior via the Ingress NGINX
CORS annotations so it is enforced by the gateway.

#### Access Logging

The agentgateway emitter supports projecting access logging behavior via:

- `nginx.ingress.kubernetes.io/enable-access-log`

This is mapped into `AgentgatewayPolicy.spec.frontend.accessLog`:

- `enable-access-log: "true"` → emit `spec.frontend.accessLog` (default access logging behavior)
- `enable-access-log: "false"` → emit `spec.frontend.accessLog.filter: "false"` (disable access logs)

**Notes:**

- ingress-nginx enables access logs by default when this annotation is absent.
- The emitter only projects access-log behavior when the annotation is explicitly present on the source Ingress.

#### Basic Authentication

The agentgateway emitter supports projecting Basic Authentication from the following Ingress NGINX annotations:

- `nginx.ingress.kubernetes.io/auth-type` (supported: `basic`)
- `nginx.ingress.kubernetes.io/auth-secret`
- `nginx.ingress.kubernetes.io/auth-secret-type` (supported: `auth-file`)

These are mapped into an `AgentgatewayPolicy` using agentgateway’s `Traffic.BasicAuthentication` model:

- `auth-secret` → `AgentgatewayPolicy.spec.traffic.basicAuthentication.secretRef.name`

**Notes:**

- The agentgateway API supports Basic Auth in two forms: inline `users` or a `secretRef`. The emitter currently
  projects only the `secretRef` form.
- `auth-secret-type` is accepted for parity with ingress-nginx. The emitter currently supports only the default
  ingress-nginx secret format: `auth-file`.
- Agentgateway expects the referenced Secret to contain a key named **`.htaccess`** with htpasswd-formatted content.
  (See the AgentgatewayPolicy API docs for details.)
- Ingress NGINX (auth-file format) typically expects htpasswd content under the key **`auth`** in the Secret.
  To support *both* dataplanes using the *same* Secret name, create a “dual-key” Secret containing **both**
  keys with the same htpasswd content:

  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: basic-auth
    namespace: default
  type: Opaque
  stringData:
    auth: |
      user:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=
    .htaccess: |
      user:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=
  ```

  This allows the same `nginx.ingress.kubernetes.io/auth-secret: basic-auth` reference to work for both
  ingress-nginx and agentgateway outputs.

#### External Authentication

The agentgateway emitter supports external authentication via:

- `nginx.ingress.kubernetes.io/auth-url`
- `nginx.ingress.kubernetes.io/auth-response-headers`

These are mapped into an `AgentgatewayPolicy` using agentgateway’s `Traffic.ExtAuth` model:

- `auth-url` → `AgentgatewayPolicy.spec.traffic.extAuth.backendRef`
  - Parsed from the URL host (must be a Kubernetes Service `*.svc[.cluster.local]`)
  - Namespace is taken from the hostname when present (`<svc>.<ns>.svc...`), otherwise defaults to the Ingress namespace
  - Port is taken from the URL when present; otherwise defaults to `80` for `http` and `443` for `https`
- `auth-url` path (when not `/`) → `AgentgatewayPolicy.spec.traffic.extAuth.http.path`
- `auth-response-headers` → `AgentgatewayPolicy.spec.traffic.extAuth.http.allowedResponseHeaders[]`

**Notes:**

- Only **in-cluster** auth URLs that resolve to a Kubernetes Service are supported. If `auth-url` does not include a
  `*.svc` hostname (i.e. it appears to be an external hostname), the provider records the intent but the emitter does
  not project an `AgentgatewayPolicy` for it.
- The emitted `spec.traffic.extAuth.http.path` is a **CEL expression** (not a plain string). For a constant path, the
  emitter emits a CEL string literal, which will look like this in YAML:

  ```yaml
  http:
    path: '"/authz"'
  ```

  This is expected: the outer quotes are YAML, and the inner quotes are part of the CEL string literal.
- If the URL path is empty or `/`, the emitter omits `http.path` (agentgateway defaults to the original request path).
- The emitter currently projects ext auth using the agentgateway **HTTP** ext auth mode (`spec.traffic.extAuth.http`).

#### Request Timeouts

The agentgateway emitter currently supports projecting request timeouts based on the following Ingress NGINX annotations:

- `nginx.ingress.kubernetes.io/proxy-send-timeout`
- `nginx.ingress.kubernetes.io/proxy-read-timeout`

These are mapped into an `AgentgatewayPolicy` using agentgateway’s `Traffic.Timeouts` model:

- `proxy-send-timeout` → `AgentgatewayPolicy.spec.traffic.timeouts.request`
- `proxy-read-timeout` → `AgentgatewayPolicy.spec.traffic.timeouts.request`

**Notes:**

- If **both** annotations are set, the emitter uses the **larger** of the two values for
  `spec.traffic.timeouts.request` to avoid unexpectedly truncating requests.
- Invalid/unsupported duration values are ignored by the provider and will not be projected.

#### Local Rate Limiting

The agentgateway emitter currently supports projecting local rate limiting via:

- `nginx.ingress.kubernetes.io/limit-rps`
- `nginx.ingress.kubernetes.io/limit-rpm`
- `nginx.ingress.kubernetes.io/limit-burst-multiplier`

These are mapped into an `AgentgatewayPolicy` using agentgateway’s `LocalRateLimit` model:

- `limit-rps` → `LocalRateLimit{ requests: <limit>, unit: Seconds }`
- `limit-rpm` → `LocalRateLimit{ requests: <limit>, unit: Minutes }`
- `limit-burst-multiplier` (when > 1) → `LocalRateLimit{ burst: limit * multiplier }`

**Notes:**

- Burst multiplier defaults to `1` if unset/zero.
- Unknown/unsupported units are ignored.

### Backend Behavior

#### Backend TLS

The agentgateway emitter supports projecting backend TLS/mTLS behavior based on the following Ingress NGINX annotations
(as represented by the provider's BackendTLS policy IR):

- `nginx.ingress.kubernetes.io/proxy-ssl-secret`
- `nginx.ingress.kubernetes.io/proxy-ssl-server-name`
- `nginx.ingress.kubernetes.io/proxy-ssl-name`
- `nginx.ingress.kubernetes.io/proxy-ssl-verify`

These are mapped into an `AgentgatewayPolicy` using agentgateway’s `BackendSimple.TLS` model:

- `proxy-ssl-name` → `AgentgatewayPolicy.spec.backend.tls.sni`
- `proxy-ssl-verify: "off"` → `AgentgatewayPolicy.spec.backend.tls.insecureSkipVerify = All`
- `proxy-ssl-secret` → `AgentgatewayPolicy.spec.backend.tls.mtlsCertificateRef[0].name`

**Notes:**

- **Per-Service policy:** Backend TLS is emitted as **one AgentgatewayPolicy per referenced backend Service**, not per Ingress.
  The policy targets the Service so that TLS settings apply when connecting to that backend.
- **SNI enablement:** ingress-nginx commonly gates sending SNI on `proxy-ssl-server-name: "on"`.
  The agentgateway emitter records SNI via `proxy-ssl-name` and projects it to `spec.backend.tls.sni`.
  If `proxy-ssl-server-name` is present in the source Ingress, it is treated as an enablement flag; the SNI value still
  comes from `proxy-ssl-name`.
- **Secret namespace handling:** If `proxy-ssl-secret` is provided as `namespace/name`, the emitter uses only `name`.
  (The referenced Secret must exist in the same namespace as the emitted policy.)
- **Verification behavior:** When `proxy-ssl-verify` disables verification, the emitter sets
  `spec.backend.tls.insecureSkipVerify: All`. When verification is enabled, the emitter does **not** set
  `insecureSkipVerify` and instead configures mTLS via `mtlsCertificateRef` (when a secret is provided).
- **CA certificates/SAN pinning:** The current mapping projects the core knobs needed for common ingress-nginx usage
  (mTLS secret, SNI, verify on/off). It does not currently project CA certificate ConfigMaps or SAN pinning fields
  (e.g. `caCertificateRefs`, `verifySubjectAltNames`) even though the agentgateway API supports them.

#### Proxy Connect Timeout

The agentgateway emitter supports projecting upstream **connection timeout** behavior via:

- `nginx.ingress.kubernetes.io/proxy-connect-timeout`

This is projected into a **Service-targeted** `AgentgatewayPolicy` by setting:

- `AgentgatewayPolicy.spec.backend.tcp.connectTimeout`

Mappings:

- `proxy-connect-timeout: "5s"` → `AgentgatewayPolicy.spec.backend.tcp.connectTimeout: 5s`

**Notes:**

- This feature is emitted as a **per-Service** `AgentgatewayPolicy` (similar to how the kgateway emitter uses a
  per-Service `BackendConfigPolicy`), because connect timeouts are backend-scoped.
- The policy targets the covered Service backends using `spec.targetRefs`:

  ```yaml
  spec:
    targetRefs:
      - group: ""
        kind: Service
        name: <service-name>
    backend:
      tcp:
        connectTimeout: 5s
  ```

- **Interaction with request timeout:** if a route-level request timeout is also configured via
  `nginx.ingress.kubernetes.io/proxy-read-timeout`/`nginx.ingress.kubernetes.io/proxy-send-timeout`, the emitter
  only projects `proxy-connect-timeout` when it is **less than or equal to**
  `AgentgatewayPolicy.spec.traffic.timeouts.request`. This mirrors the behavior used in agentgateway output to avoid
  producing a connect timeout that exceeds the effective request timeout.
- Invalid/unsupported duration values are ignored by the provider and will not be projected.

#### Backend Protocol

The agentgateway emitter supports projecting gRPC upstream protocol selection via:

- `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"` (or `"GRPCS"`)

This is projected into a **Service-targeted** `AgentgatewayPolicy` by setting:

- `AgentgatewayPolicy.spec.backend.http.version: HTTP2`

**Notes:**

- This feature is emitted as a **per-Service** `AgentgatewayPolicy` because backend HTTP protocol selection is
  backend-scoped.
- This annotation does **not** cause ingress2gateway to emit a `GRPCRoute`. The generated route remains an
  `HTTPRoute`; the emitter uses `AgentgatewayPolicy.spec.backend.http.version: HTTP2` to express how agentgateway
  should communicate with the upstream backend.
- HTTP/HTTPS/AUTO_HTTP are treated as default behavior by the provider and are not projected.
- The provider currently maps only gRPC-family values into policy IR, so the agentgateway emitter currently emits
  only `HTTP2` for this feature.

## AgentgatewayPolicy Projection

Rate limit, timeout, CORS, rewrite target, access log, etc. annotations are converted into AgentgatewayPolicy resources.
The agentgateway emitter emits AgentgatewayPolicy resources in two shapes:

- HTTPRoute-scoped policies for traffic-level behavior (rate limit, request timeouts, CORS, rewrite target,
  basic auth, ext auth, access log).
- Service-scoped policies for backend connection behavior (backend TLS, proxy connect timeout, backend protocol).

### Naming

Policies are created **per source Ingress name**:

- `metadata.name: <ingress-name>`
- `metadata.namespace: <route-namespace>`

Backend TLS policies are created **per backend Service**:

- `metadata.name: <service-name>-backend-tls`
- `metadata.namespace: <route-namespace>`

Proxy connect timeout policies are created **per backend Service**:

- `metadata.name: <service-name>-backend-connect-timeout`
- `metadata.namespace: <route-namespace>`

Backend protocol policies are created **per backend Service**:

- `metadata.name: <service-name>-backend-http-version`
- `metadata.namespace: <route-namespace>`

### Attachment Semantics

If a policy covers all backends of the generated HTTPRoute, the policy is attached using `spec.targetRefs`
to the HTTPRoute.

If a policy only covers some (rule, backendRef) pairs, the emitter **returns an error** and does not emit
+agentgateway resources for that Ingress.

Conceptually:

- **Full coverage** → `AgentgatewayPolicy.spec.targetRefs[]` references the HTTPRoute
- **Partial coverage** → **error** (agentgateway does not support attaching `AgentgatewayPolicy` via per-backend
  `HTTPRoute` `ExtensionRef` filters)

#### Why?

Agentgateway does not support `HTTPRoute` `backendRefs[].filters[].type: ExtensionRef` for attaching policies.
Attempting to generate per-backend `ExtensionRef` filters results in `HTTPRoute` status failures (e.g.
`ResolvedRefs=False` with an `IncompatibleFilters` error). To avoid emitting manifests that will be rejected or
non-functional at runtime, the emitter fails fast during generation when only partial attachment is possible.

#### Workarounds

- Split the source Ingress into separate Ingress resources so each generated HTTPRoute can be fully covered by a policy.
- Adjust annotations so the policy applies uniformly to all paths/backends of the resulting HTTPRoute.

For backend TLS, prefer targeting the **Service** via a backend policy (as emitted) so TLS settings apply cleanly without
needing per-backend HTTPRoute filters.

## Deterministic Output

For stable golden tests, agentgateway extension objects are sorted (Kind, Namespace, Name) before being appended
to the output extensions list.

## Limitations

- Only the **ingress-nginx provider** is currently supported by the Agentgateway emitter.
- Regex path matching is not currently implemented for agentgateway output.

## Future Work

The code defines GVKs for additional agentgateway extension types (e.g. `AgentgatewayBackend`), but they are not
yet emitted by the current implementation.
