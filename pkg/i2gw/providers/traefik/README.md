# Traefik Provider

The project supports translating [Traefik](https://traefik.io/) specific annotations into Gateway API resources.

## Path Type Handling

| Ingress `pathType` | Gateway API match type |
|---|---|
| `Prefix` | `PathMatchPathPrefix` |
| `Exact` | `PathMatchExact` |
| `ImplementationSpecific` | `PathMatchPathPrefix` (Traefik's default behaviour) |

## Supported Annotations

### TLS

- `traefik.ingress.kubernetes.io/router.tls`: When set to `true` and no `spec.tls` block is present in the Ingress, adds an HTTPS listener (port 443, `Terminate` mode) to the generated Gateway. A conventional placeholder secret name is generated from the hostname — `{hostname-with-dashes}-tls` (e.g. `my-app-example-com-tls`) — and used as the `certificateRef`. Create this secret (e.g. via cert-manager) before applying the output.

### Entrypoints

- `traefik.ingress.kubernetes.io/router.entrypoints`: Controls which Traefik entrypoints (ports) the router listens on. The two standard values are:
  - `websecure` only — removes the HTTP listener (port 80) from the generated Gateway; the route becomes HTTPS-only.
  - `web` only — removes the HTTPS listener (port 443) from the generated Gateway; the route becomes HTTP-only.
  - `web,websecure` — no listeners are removed; this matches the default Gateway API behaviour.
  - Any other (non-standard) entrypoint name — **Recognized but not converted.** A warning is emitted asking you to review the Gateway listener configuration manually.

### Not Converted (warnings emitted)

- `traefik.ingress.kubernetes.io/router.middlewares`: **Recognized but not converted.** Traefik Middlewares are CRDs with no direct Gateway API equivalent. A warning is emitted. Consider using implementation-specific policy attachments (e.g. `ExtensionRef` filters) supported by your Gateway implementation.

- `traefik.ingress.kubernetes.io/router.priority`: **Recognized but not converted.** Traefik router priority has no direct Gateway API equivalent. A warning is emitted. Use HTTPRoute rule ordering for match precedence instead.

If you are reliant on any annotations not listed above, please open an issue. In the meantime you'll need to manually find a Gateway API equivalent.

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
  - name: my-app-example-com-https   # HTTP listener removed (entrypoints: websecure)
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
