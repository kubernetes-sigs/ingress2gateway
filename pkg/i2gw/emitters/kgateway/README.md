# Kgateway Emitter

The Kgateway Emitter supports generating **Gateway API** and **Kgateway** resources from Ingress manifests using:

- **Provider**: `ingress-nginx`

**Note:** All other providers will be ignored by the emitter.

## Development Workflow

The typical development workflow for adding an Ingress NGINX feature to the Kgateway emitter is:

1. Use [this issue](https://github.com/kubernetes-sigs/ingress2gateway/issues/232) to prioritize the list of Ingress NGINX features unless
   the business provides requirements. When this list is complete, refer to [this doc](https://docs.google.com/document/d/12ejNTb45hASGYvUjd3t9mfNwuMTX3KBM68ALM4jRFBY/edit?usp=sharing) for additional features. **Note:** Several of the features from the above list have already been implemented, so review the
   current supported features before adding more.
2. If any of the above features cannot map to an existing Kgateway API, create a Kgateway issue, label it with `kind/ingress-nginx`,
   `help wanted`, `priority/high`, etc. and describe what's needed.
3. Extend the ingress-nginx emitter IR (`pkg/i2gw/emitter_intermediate/intermediate_representation.go`) as needed. Most changes should fall within the Policy IR.
4. Add a feature-specific function to the ingress-nginx provider (`pkg/i2gw/providers/ingressnginx`), e.g. `proxyReadTimeoutFeature()`
   that parses the Ingress NGINX annotation from source Ingresses and records them as ingress-nginx policy IR that is converted into emitter IR.
5. Update the Kgateway Emitter (`pkg/i2gw/emitters/kgateway/emitter.go`) to consume the IR and return Kgateway-specific resources.
6. Follow the **Testing** section to test your changes.
7. Update the list of supported annotations with the feature you added.
8. Submit a PR to merge your changes upstream. [This branch](https://github.com/danehans/ingress2gateway/tree/impl_emitter_nginx_feat) is the **current** upstream, but [k8s-sigs](https://github.com/kubernetes-sigs/ingress2gateway) or [solo](https://github.com/solo-io/ingress2gateway) repos should be used before releasing.

## Testing

Run the tool with the test input manifest:

```bash
go run . print \
  --providers=ingress-nginx \
  --emitter=kgateway \
  --input-file ./pkg/i2gw/emitters/kgateway/testing/testdata/input.yaml
```

The command should generate Gateway API and Kgateway resources.

## Supported Annotations

### Traffic Behavior

- `nginx.ingress.kubernetes.io/client-body-buffer-size`
- `nginx.ingress.kubernetes.io/proxy-body-size`
- `nginx.ingress.kubernetes.io/enable-cors`
- `nginx.ingress.kubernetes.io/cors-allow-origin`
- `nginx.ingress.kubernetes.io/cors-allow-credentials`
- `nginx.ingress.kubernetes.io/cors-allow-headers`
- `nginx.ingress.kubernetes.io/cors-expose-headers`
- `nginx.ingress.kubernetes.io/cors-allow-methods`
- `nginx.ingress.kubernetes.io/cors-max-age`
- `nginx.ingress.kubernetes.io/limit-rps`
- `nginx.ingress.kubernetes.io/limit-rpm`
- `nginx.ingress.kubernetes.io/limit-burst-multiplier`
- `nginx.ingress.kubernetes.io/proxy-send-timeout`
- `nginx.ingress.kubernetes.io/proxy-read-timeout`
- `nginx.ingress.kubernetes.io/ssl-redirect`: When set to `"true"`, adds a `RequestRedirect` filter to HTTPRoute rules that redirects HTTP to HTTPS with a 301 status code. Note that ingress-nginx redirects with code 308, but that isn't supported by gateway API. 
- `nginx.ingress.kubernetes.io/force-ssl-redirect`: When set to `"true"`, adds a `RequestRedirect` filter to HTTPRoute rules that redirects HTTP to HTTPS with a 301 status code. Treated identically to `ssl-redirect`. Note that ingress-nginx redirects with code 308, but that isn't supported by gateway API.
- `nginx.ingress.kubernetes.io/ssl-passthrough`: When set to `"true"`, enables TLS passthrough mode. Converts the Ingress to a `TLSRoute` with a Gateway listener using `protocol: TLS` and `tls.mode: Passthrough`. The HTTPRoute that would normally be created is removed.
- `nginx.ingress.kubernetes.io/use-regex`: When set to `"true"`, indicates that the paths defined on an Ingress should be treated as regular expressions.
  Uses host-group semantics: if any Ingress contributing rules for a given host has `use-regex: "true"`, regex-style path matching is enforced on **all**
  paths for that host (across all contributing Ingresses).
- `nginx.ingress.kubernetes.io/rewrite-target`: Rewrites the request path using regex rewrite semantics.
  Uses host-group semantics: if any Ingress contributing rules for a given host sets `rewrite-target`, regex-style path matching is enforced on **all**
  paths for that host (across all contributing Ingresses), consistent with ingress-nginx behavior.

### Backend Behavior

- `nginx.ingress.kubernetes.io/proxy-connect-timeout`: Sets the timeout for establishing a connection with a proxied server. It should be noted that this timeout
  cannot usually exceed 75 seconds.
- `nginx.ingress.kubernetes.io/load-balance`: Sets the algorithm to use for load balancing to a proxied server. The only supported value is `round_robin`.
- `nginx.ingress.kubernetes.io/affinity`: Enables session affinity (only "cookie" type is supported). Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies`.
- `nginx.ingress.kubernetes.io/session-cookie-name`: Specifies the name of the cookie used for session affinity. Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.name`.
- `nginx.ingress.kubernetes.io/session-cookie-path`: Defines the path that will be set on the cookie. Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.path`.
- **Note (regex-mode constraint):** Ingress NGINX session cookie paths do not support regex. If regex-mode is enabled for a host (via `use-regex: "true"` or
  `rewrite-target`) and cookie affinity is used, `session-cookie-path` must be set; the provider validates this and emits an error if it is missing.
- `nginx.ingress.kubernetes.io/session-cookie-domain`: Sets the Domain attribute of the sticky cookie. **Note:** This annotation is parsed but not currently mapped to kgateway as the Cookie type doesn't support domain.
- `nginx.ingress.kubernetes.io/session-cookie-samesite`: Applies a SameSite attribute to the sticky cookie. Browser accepted values are None, Lax, and Strict. Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.sameSite`.
- `nginx.ingress.kubernetes.io/session-cookie-expires`: Sets the TTL/expiration time for the cookie. Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.ttl`.
- `nginx.ingress.kubernetes.io/session-cookie-max-age`: Sets the TTL/expiration time for the cookie (takes precedence over `session-cookie-expires`). Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.ttl`.
- `nginx.ingress.kubernetes.io/session-cookie-secure`: Sets the Secure flag on the cookie. Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies[].cookie.secure`.
- `nginx.ingress.kubernetes.io/service-upstream`: When set to `"true"`, configures Kgateway to route to the Service’s cluster IP (or equivalent static host) instead of individual Pod IPs. For each covered Service, the emitter creates a `Backend` resource with `spec.type: Static` and rewrites the corresponding `HTTPRoute.spec.rules[].backendRefs[]` to reference that `Backend` (group `gateway.kgateway.dev`, kind `Backend`).
- `nginx.ingress.kubernetes.io/backend-protocol`: Indicates the L7 protocol that is used to communicate with the proxied backend.
  - This annotation affects **upstream/backend** protocol selection only. It does **not** cause ingress2gateway to emit
    a `GRPCRoute`; the generated route remains an `HTTPRoute`, and the emitter projects backend protocol intent onto
    backend-facing configuration.
  - **Supported values (mapped):** `GRPC`, `GRPCS`
    - If `service-upstream: "true"` is also set for the same Service backend, the emitter sets `spec.static.appProtocol: grpc` on the generated `Backend`.
    - Otherwise, the emitter does **not** create or modify Kubernetes `Service` resources. Instead, it emits an **INFO** notification with a `kubectl patch`
      command to update the existing Service port with `appProtocol: grpc`.
  - **Values treated as default HTTP/1.x (no-op):** `HTTP`, `HTTPS`, `AUTO_HTTP`
  - **Unsupported values (rejected by provider):** `FCGI` (and others)
  - **Safety note:** Because emitting Service manifests could overwrite user-managed Service configuration, ingress2gateway intentionally avoids generating
    Service resources for this annotation.

### External Auth

- `nginx.ingress.kubernetes.io/auth-url`: Specifies the URL of an external authentication service.
- `nginx.ingress.kubernetes.io/auth-response-headers`: Comma-separated list of headers to pass to backend once authentication request completes.

### Basic Auth

- `nginx.ingress.kubernetes.io/auth-type`: Must be set to `"basic"` to enable basic authentication. Maps to `TrafficPolicy.spec.basicAuth`.
- `nginx.ingress.kubernetes.io/auth-secret`: Specifies the secret containing basic auth credentials in `namespace/name` format (or just `name` if in the same namespace). Maps to `TrafficPolicy.spec.basicAuth.secretRef.name`.
- `nginx.ingress.kubernetes.io/auth-secret-type`: **Only `"auth-file"` is supported** (default). The secret must contain an htpasswd file in the key `"auth"`. Only SHA hashed passwords are supported. Maps to `TrafficPolicy.spec.basicAuth.secretRef.key` set to `"auth"`.

### Backend TLS

- `nginx.ingress.kubernetes.io/proxy-ssl-secret`: Maps to `BackendConfigPolicy.spec.tls.secretRef`
- `nginx.ingress.kubernetes.io/proxy-ssl-verify`: Maps to `BackendConfigPolicy.spec.tls.insecureSkipVerify` (inverted: `"on"` = `false`, `"off"` = `true`)
- `nginx.ingress.kubernetes.io/proxy-ssl-name`: Maps to `BackendConfigPolicy.spec.tls.sni` (automatically enables SNI)

### Access Logging

- `nginx.ingress.kubernetes.io/enable-access-log`: If enabled, will create an HTTPListenerPolicy that will configure a basic policy for envoy access logging. Maps to `HTTPListenerPolicy.spec.accessLog[].fileSink`. This can be further customized as needed, see [docs](https://kgateway.dev/docs/envoy/2.0.x/security/access-logging/).

### Regex Path Matching and Rewrites

- `use-regex` and `rewrite-target` may **mutate HTTPRoute path matching** for a host:
  - When regex-mode is enabled for a host, the emitter converts **all** `PathPrefix`/`Exact` matches under that host to `RegularExpression` matches.
  - For Ingresses that set `use-regex: "true"`, their contributed path strings are treated as **regex** (not escaped as literals).
  - For other Ingresses under the same host (that did not set `use-regex: "true"`), their contributed path strings are treated as **literals** within a regex
    match (escaped), to preserve the original non-regex intent.

- `rewrite-target` generates `TrafficPolicy` URL rewrite:
  - For each rule covered by an Ingress that sets `rewrite-target`, the emitter creates a **per-rule TrafficPolicy** named:
    - `<ingress-name>-rewrite-<rule-index>`
  - That policy sets:

    ```yaml
    spec:
      urlRewrite:
        pathRegex:
          pattern: <regex pattern derived from the HTTPRoute rule match>
          substitution: <rewrite-target value>
    ```

  - The policy is attached via `ExtensionRef` filters to only the covered backendRefs (partial coverage), rather than using `targetRefs`.

## TrafficPolicy Projection

Annotations in the **Traffic Behavior** category are converted into
`TrafficPolicy` resources.

These policies are attached using:

- `targetRefs` when the policy applies to all backends, or
- `extensionRef` backend filters for partial coverage.

Examples:

- Body size annotations control `spec.buffer.maxRequestSize`
- Rate limit annotations control `spec.rateLimit.local.tokenBucket`
- Timeout annotations control `spec.timeouts.request` or `streamIdle`
- SSL redirect annotations add `RequestRedirect` filters to HTTPRoute rules
- SSL passthrough annotation converts HTTPRoute to TLSRoute with TLS passthrough Gateway listener

## BackendConfigPolicy Projection

Annotations in the **Backend Behavior** category are converted into
`BackendConfigPolicy` resources.

Currently supported:

- `proxy-connect-timeout`: Maps to `BackendConfigPolicy.spec.connectTimeout`
- Session affinity annotations: Maps to `BackendConfigPolicy.spec.loadBalancer.ringHash.hashPolicies` with cookie-based hash policy

If multiple Ingresses target the same Service with conflicting `proxy-connect-timeout` values,
the lowest timeout wins and a warning is emitted.

## TLSRoute Projection

Annotations that require TLS passthrough mode are converted into `TLSRoute` resources instead of `HTTPRoute` resources.

Currently supported:

- `nginx.ingress.kubernetes.io/ssl-passthrough`:
  - When enabled, the Ingress is converted to a `TLSRoute` resource
  - A Gateway listener is created with `protocol: TLS` and `tls.mode: Passthrough`
  - The listener uses port 443 (when hostname is specified) or 8443 (default)
  - The HTTPRoute that would normally be created is removed
  - Backend services must handle TLS termination themselves

## Backend Projection

Annotations that change how upstreams are represented (rather than how they are load balanced or configured)
can be projected into Kgateway `Backend` resources.

Currently supported:

- `nginx.ingress.kubernetes.io/service-upstream`:
  - For each Service backend covered by an Ingress with `service-upstream: "true"`, the emitter creates a `Backend` with:
    - `spec.type: Static`
    - `spec.static.hosts` containing a single `{host, port}` entry derived from the Service (e.g. `myservice.default.svc.cluster.local:80`).
  - Matching `HTTPRoute.spec.rules[].backendRefs[]` are rewritten to reference this `Backend` instead of the core Service.
- `nginx.ingress.kubernetes.io/backend-protocol`:
  - This annotation does **not** switch route generation from `HTTPRoute` to `GRPCRoute`; it only influences backend
    connection/protocol behavior.
  - When set to `GRPC` or `GRPCS` **and** `service-upstream: "true"` is set for the same backend, the emitter stamps `spec.static.appProtocol: grpc` on the generated `Backend`.
  - When set to `GRPC` or `GRPCS` **without** `service-upstream: "true"`, the emitter emits an **INFO** notification that includes a `kubectl patch service ...`
    command to set `spec.ports[].appProtocol` on the existing Service.
  - This annotation controls upstream/backend protocol selection only; the emitter keeps generated routes as `HTTPRoute` and does not project `GRPCRoute` from this annotation.
  - `HTTP`, `HTTPS`, and `AUTO_HTTP` are treated as default HTTP/1.x behavior and do not emit additional config. 

### Summary of Policy Types

| Annotation Type                    | Kgateway Resource     |
|------------------------------------|-----------------------|
| Request/response behavior          | `TrafficPolicy`       |
| Upstream connection behavior       | `BackendConfigPolicy` |
| Upstream representation.           | `Backend`             |
| TLS passthrough                    | `TLSRoute`            |

## Limitations

- Only the **ingress-nginx provider** is currently supported by the Kgateway emitter.
- Some NGINX behaviors cannot be reproduced exactly due to Envoy/Kgateway differences.
- Regex-mode is implemented by converting HTTPRoute path matches to `RegularExpression`. Some ingress-nginx details (such as case-insensitive `~*` behavior)
  may not be reproduced exactly depending on the underlying Gateway API/Envoy behavior and the patterns provided.

## Supported but not translated Annotations

The following annotations have equivalents in kgateway but are not (as of yet) translated by this tool.

`nginx.ingress.kubernetes.io/auth-proxy-set-headers`

Supported in TrafficPolicy

```yaml
spec:
  extAuth:
    httpService:
      authorizationRequest:
        headersToAdd:
        - key: x-forwarded-host
          value: "%DOWNSTREAM_REMOTE_ADDRESS%"
```
