# NGINX Ingress Controller Provider

This provider converts [NGINX Ingress Controller](https://github.com/nginx/kubernetes-ingress) resources to Gateway API resources.

**Note**: This provider is specifically for NGINX Ingress Controller, not the community [ingress-nginx](https://github.com/kubernetes/ingress-nginx) controller. If you're using the community ingress-nginx controller, please use the `ingress-nginx` provider instead.

## Supported Resources

* **Ingress** - Core Kubernetes Ingress resources with NGINX-specific annotations
* **Service** - Kubernetes Services referenced by Ingress backend configurations

## Supported Annotations

* `nginx.org/ssl-services` - SSL/TLS backend connections
* `nginx.org/grpc-services` - gRPC backend connections  
* `nginx.org/websocket-services` - WebSocket backend connections
* `nginx.org/proxy-hide-headers` - Hide headers from responses
* `nginx.org/proxy-set-headers` - Set custom headers
* `nginx.org/listen-ports` - Custom HTTP ports
* `nginx.org/listen-ports-ssl` - Custom HTTPS ports
* `nginx.org/path-regex` - Regex path matching
* `nginx.org/rewrites` - URL rewriting
* `nginx.org/redirect-to-https` - SSL/HTTPS redirects
* `ingress.kubernetes.io/ssl-redirect` - Legacy SSL redirect (for compatibility)
* `nginx.org/hsts` - HTTP Strict Transport Security headers
* `nginx.org/hsts-max-age` - HSTS max-age directive
* `nginx.org/hsts-include-subdomains` - HSTS includeSubDomains directive

## Usage

```bash
# Convert NGINX Ingress Controller resources from cluster
ingress2gateway print --providers=nginx

# Convert from file
ingress2gateway print --providers=nginx --input-file=nginx-ingress.yaml
```

## Gateway API Mapping

| NGINX Annotation                    | Gateway API Resource              |
|--------------------------------------|-----------------------------------|
| `nginx.org/ssl-services`             | BackendTLSPolicy                  |
| `nginx.org/grpc-services`            | GRPCRoute                         |
| `nginx.org/websocket-services`       | Informational notification only  |
| `nginx.org/proxy-hide-headers`       | HTTPRoute ResponseHeaderModifier  |
| `nginx.org/proxy-set-headers`        | HTTPRoute RequestHeaderModifier   |
| `nginx.org/rewrites`                 | HTTPRoute URLRewrite filter       |
| `nginx.org/listen-ports*`            | Gateway custom listeners          |
| `nginx.org/path-regex`               | HTTPRoute RegularExpression paths |
| `nginx.org/redirect-to-https`        | HTTPRoute RequestRedirect filter  |
| `ingress.kubernetes.io/ssl-redirect` | HTTPRoute RequestRedirect filter  |
| `nginx.org/hsts*`                    | HTTPRoute ResponseHeaderModifier  |

## SSL Redirect Behavior

The provider supports two SSL redirect annotations with identical behavior:

* **`nginx.org/redirect-to-https`** - Redirects all HTTP traffic to HTTPS with a 301 status code
* **`ingress.kubernetes.io/ssl-redirect`** - Redirects all HTTP traffic to HTTPS with a 301 status code (legacy compatibility)

## Contributing

When adding support for new NGINX Ingress Controller annotations:

1. Add the annotation constant to `annotations/constants.go`
2. Implement the conversion logic in the appropriate `annotations/*.go` file
3. Add comprehensive tests in `annotations/*_test.go`
4. Update this README with the new annotation details

For more information on the provider architecture, see [PROVIDER.md](../../PROVIDER.md).

## References

* [NGINX Ingress Controller](https://github.com/nginx/kubernetes-ingress)
* [NGINX Ingress Controller Annotations](https://docs.nginx.com/nginx-ingress-controller/configuration/ingress-resources/advanced-configuration-with-annotations/)
* [NGINX Gateway Fabric](https://docs.nginx.com/nginx-gateway-fabric/)
* [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)