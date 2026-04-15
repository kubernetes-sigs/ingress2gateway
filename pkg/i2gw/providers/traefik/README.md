# Traefik Provider

The project supports translating [Traefik](https://traefik.io/) specific annotations into Gateway API resources.

## Path Type Handling

| Ingress `pathType` | Gateway API match type |
|---|---|
| `Prefix` | `PathMatchPathPrefix` |
| `Exact` | `PathMatchExact` |
| `ImplementationSpecific` | `PathMatchPathPrefix` (Traefik's default behaviour) |

## Supported Annotations

Traefik exposes a large number of [ingress annotations](https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/ingress/). Only the annotations listed below are supported by this provider. Any other annotation starting with `traefik.ingress.kubernetes.io/` is **not converted** — a warning is emitted and you will need to find a Gateway API equivalent manually.

### TLS

- `traefik.ingress.kubernetes.io/router.tls`: When set to `true` and no `spec.tls` block is present in the Ingress, adds an HTTPS listener (port 443, `Terminate` mode) to the generated Gateway. A conventional placeholder secret name is generated from the hostname — `{hostname-with-dashes}-tls` (e.g. `my-app-example-com-tls`) — and used as the `certificateRef`. Create this secret (e.g. via cert-manager) before applying the output.

### Entrypoints

- `traefik.ingress.kubernetes.io/router.entrypoints`: Controls which Traefik entrypoints (ports) the router listens on. The two standard values are:
  - `websecure` only — the route is HTTPS-only. If an HTTPS listener (port 443) is present on the generated Gateway, the HTTP listener (port 80) is **kept** and an HTTP→HTTPS redirect HTTPRoute (301) is generated and bound to it, mirroring Traefik's behaviour. If no HTTPS listener exists (e.g. `router.tls` is not set and no `spec.tls` block is present), the HTTP listener is removed instead.
  - `web` only — removes the HTTPS listener (port 443) from the generated Gateway; the route becomes HTTP-only.
  - `web,websecure` — no listeners are removed; this matches the default Gateway API behaviour.
  - Any other (non-standard) entrypoint name — **Recognized but not converted.** A warning is emitted asking you to review the Gateway listener configuration manually.

### HTTP→HTTPS Redirect

When `router.entrypoints: websecure` is combined with `router.tls: "true"` (or a `spec.tls` block), ingress2gateway generates an additional redirect HTTPRoute named `{route-name}-http` that redirects all HTTP traffic (port 80) to HTTPS (301). This HTTPRoute is bound to the HTTP listener via `sectionName` so that only port-80 traffic is affected. The main HTTPRoute continues to serve HTTPS traffic on port 443.

## Annotation Interaction Order

When multiple annotations are present, they are applied in this order:

1. **`spec.tls`** (standard Ingress field) — processed first by the common converter. If a `spec.tls` block is present, an HTTPS listener is already created with the real secret name before any Traefik annotations are evaluated.

2. **`router.tls`** — evaluated only when `spec.tls` is **absent**. If `spec.tls` is present, `router.tls` is a no-op — the existing HTTPS listener is used as-is and no duplicate listener is created.

3. **`router.entrypoints`** — evaluated after TLS listeners are in place. When `websecure` is set:
   - If an HTTPS listener exists (from either `spec.tls` or `router.tls`), the HTTP listener is kept and a redirect HTTPRoute is generated.
   - If no HTTPS listener exists, the HTTP listener is removed.

**In practice, the common combinations are:**

| `spec.tls` | `router.tls` | `router.entrypoints` | Result |
|---|---|---|---|
| present | any | `websecure` | HTTP + HTTPS listeners (real secret from `spec.tls`) + HTTP→HTTPS redirect route |
| absent | `true` | `websecure` | HTTP + HTTPS listeners (placeholder secret) + HTTP→HTTPS redirect route |
| absent | `true` | absent | HTTP + HTTPS listeners (placeholder secret), no redirect |
| absent | absent | `websecure` | HTTP listener removed (no HTTPS listener to redirect to) |
| absent | absent | `web` | HTTP listener only (no HTTPS listener was present to remove) |

### Not Converted (warnings emitted)

Any annotation starting with `traefik.ingress.kubernetes.io/` that is not listed above is not converted and will emit a warning. If you rely on an annotation that is not supported, please [open an issue](https://github.com/kubernetes-sigs/ingress2gateway/issues). In the meantime you'll need to manually find a Gateway API equivalent.

## Example

Input Ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: production
  annotations:
    traefik.ingress.kubernetes.io/router.tls: "true"
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
spec:
  ingressClassName: traefik
  rules:
  - host: my-app.example.com
    http:
      paths:
      - path: /
        pathType: ImplementationSpecific
        backend:
          service:
            name: my-app
            port:
              number: 8080
```

Run:

```bash
ingress2gateway print --providers=traefik --input-file=ingress.yaml
```

Output:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: traefik
  namespace: production
spec:
  gatewayClassName: traefik
  listeners:
  - name: my-app-example-com-http    # HTTP listener kept for redirect
    hostname: my-app.example.com
    port: 80
    protocol: HTTP
  - name: my-app-example-com-https
    hostname: my-app.example.com
    port: 443
    protocol: HTTPS
    tls:
      mode: Terminate
      certificateRefs:
      - group: ""
        kind: Secret
        name: my-app-example-com-tls  # create this secret before applying
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-app-my-app-example-com-http   # redirects HTTP → HTTPS (301)
  namespace: production
spec:
  parentRefs:
  - name: traefik
    namespace: production
    sectionName: my-app-example-com-http
  hostnames:
  - my-app.example.com
  rules:
  - filters:
    - type: RequestRedirect
      requestRedirect:
        scheme: https
        statusCode: 301
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-app-my-app-example-com
  namespace: production
spec:
  parentRefs:
  - name: traefik
  hostnames:
  - my-app.example.com
  rules:
  - matches:
    - path:
        type: PathPrefix           # ImplementationSpecific → PathPrefix
        value: /
    backendRefs:
    - name: my-app
      port: 8080
```
