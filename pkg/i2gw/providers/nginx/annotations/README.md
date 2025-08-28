# NGINX Ingress Annotations

This directory contains the implementation of [NGINX Ingress Controller](https://github.com/nginx/kubernetes-ingress) annotations for the ingress2gateway conversion tool.

**Note**: This is specifically for NGINX Ingress Controller, not the community [ingress-nginx](https://github.com/kubernetes/ingress-nginx) controller.

## Structure

- **`constants.go`** - All annotation constants and schema definitions
- **`ssl_services.go`** - SSL backend services (`ssl-services`)
- **`grpc_services.go`** - gRPC backend services (`grpc-services`)
- **`websocket_services.go`** - WebSocket backend services (`websocket-services`)
- **`header_manipulation.go`** - Header manipulation annotations (`hide-headers`, `proxy-set-headers`, etc.)
- **`hsts.go`** - HSTS header annotations (`hsts`)
- **`listen_ports.go`** - Custom port listeners (`listen-ports`, `listen-ports-ssl`)
- **`path_matching.go`** - Path regex matching (`path-regex`)
- **`path_rewrite.go`** - URL rewriting (`rewrites`)
- **`ssl_redirect.go`** - SSL/HTTPS redirects (`redirect-to-https`)

## Exported Functions

Each annotation file exports a main feature function:

- `SSLServicesFeature` - Processes SSL backend services annotations
- `GRPCServicesFeature` - Processes gRPC backend services annotations
- `WebSocketServicesFeature` - Processes WebSocket backend services annotations
- `HeaderManipulationFeature` - Processes header manipulation annotations
- `HSTSFeature` - Processes HSTS header annotations
- `ListenPortsFeature` - Processes custom port listener annotations
- `PathRegexFeature` - Processes path regex annotations
- `RewriteTargetFeature` - Processes URL rewrite annotations
- `SSLRedirectFeature` - Processes SSL redirect annotations

## Testing

Each annotation implementation includes comprehensive unit tests:

- `*_test.go` files contain feature-specific tests
- `*_helpers_test.go` files contain shared test utilities
- Tests cover various annotation formats, edge cases, and error conditions

## Adding New Annotations

To add a new NGINX annotation:

1. Add the annotation constant to `constants.go`
2. Create the feature implementation file (e.g., `my_feature.go`)
3. Export the main feature function (e.g., `MyFeature`)
4. Add comprehensive tests in `my_feature_test.go`
5. Register the feature function in `../converter.go`

## Limitations and Known Issues

### Multiple Ingresses with Same Hostname

When multiple Ingress resources define rules for the same hostname (within the same namespace and ingress class), all annotations from all Ingresses will be applied to the resulting HTTPRoute. This can lead to unexpected behavior where annotations from one Ingress affect traffic intended for another.

**Example:**
```yaml
# ingress-app1.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app1
  annotations:
    nginx.org/proxy-hide-headers: "Server,X-Powered-By"
    nginx.org/proxy-set-headers: "X-App: app1"
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /app1
        backend:
          service:
            name: app1-service
            port:
              number: 80

---
# ingress-app2.yaml  
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app2
  annotations:
    nginx.org/proxy-set-headers: "X-App: app2"
    nginx.org/hsts: "true"
spec:
  rules:
  - host: example.com  # Same hostname!
    http:
      paths:
      - path: /app2
        backend:
          service:
            name: app2-service
            port:
              number: 80
```

**Result:** The converted HTTPRoute will have ALL annotations applied:
- Both `X-App: app1` AND `X-App: app2` headers will be set on all rules
- `Server,X-Powered-By` headers will be hidden for all rules
- HSTS will be enabled for all rules
- This affects both `/app1` and `/app2` paths

**Workarounds:**
1. **Separate Hostnames**: Use different hostnames for different applications (e.g., `app1.example.com`, `app2.example.com`)
2. **Single Ingress**: Combine all paths into a single Ingress resource with consistent annotations
3. **Post-Conversion Manual Editing**: Manually edit the generated HTTPRoutes to apply filters only to specific rules

This limitation exists because the rule grouping mechanism groups rules by `namespace/ingressClass/hostname`, and annotation processing applies all discovered annotations to the entire rule group.
[Issue #229](https://github.com/kubernetes-sigs/ingress2gateway/issues/229)


## Integration

These annotation handlers are integrated into the main NGINX provider via `../converter.go`, which registers all feature parsers with the conversion pipeline.