# Ingress Nginx Provider

The project supports translating ingress-nginx specific annotations.

## Ingress Class Name

To specify the name of the Ingress class to select, use `--ingress-nginx-ingress-class=ingress-nginx` (default to 'nginx').

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

- `nginx.ingress.kubernetes.io/proxy-body-size`: Sets the maximum allowed request body size. Takes precedence over `client-body-buffer-size`. For the Kgateway implementation, this maps to `TrafficPolicy.spec.buffer.maxRequestSize`.

---

### CORS

- `nginx.ingress.kubernetes.io/enable-cors`: Enables CORS policy generation.

- `nginx.ingress.kubernetes.io/cors-allow-origin`: Comma-separated list of origins. For the Kgateway implementation, this maps to `TrafficPolicy.spec.cors.allowOrigins`.

---

### Rate Limiting

- `nginx.ingress.kubernetes.io/limit-rps`: Requests per second limit.  For the Kgateway implementation, this maps to `TrafficPolicy.spec.rateLimit.local.tokenBucket`.

- `nginx.ingress.kubernetes.io/limit-rpm`: Requests per minute limit. For the Kgateway implementation, this maps to `TrafficPolicy.spec.rateLimit.local.tokenBucket`.

- `nginx.ingress.kubernetes.io/limit-burst-multiplier`: Burst multiplier for rate limiting. Used to compute `maxTokens`.

---

### Timeouts

- `nginx.ingress.kubernetes.io/proxy-send-timeout`: Controls the request timeout. For the Kgateway implementation, this maps to `TrafficPolicy.spec.timeouts.request`.

- `nginx.ingress.kubernetes.io/proxy-read-timeout`: Controls stream idle timeout. For the Kgateway implementation, this maps to `TrafficPolicy.spec.timeouts.streamIdle`.

---

### Backend (Upstream) Configuration

- `nginx.ingress.kubernetes.io/proxy-connect-timeout`: Controls the upstream connection timeout. For the Kgateway implementation, this maps to `BackendConfigPolicy.spec.connectTimeout`.

**Note:** For the Kgateway implementation, if multiple Ingress resources reference the same Service with different `proxy-connect-timeout` values, ingress2gateway will emit warnings because Kgateway cannot safely apply multiple conflicting `BackendConfigPolicy` resources to the same Service.

---

## Provider Limitations

- Currently, kgateway is the only supported implementation-specific emitter.
- Some NGINX behaviors cannot be reproduced exactly due to differences between NGINX and semantics of other proxy implementations.

If you rely on annotations not listed above, please open an issue or be prepared to apply post-migration manual adjustments.
