# Kgateway Emitter

This branch adds support for generating **Gateway API** and **Kgateway** resources from
Ingress manifests using:

- **Provider**: `ingress-nginx`

**Note:** At this time, `ingress-nginx` is the only supported provider. All other providers will be ignored by the emitter.

---

## Testing

Run the tool with the test input manifest:

```bash
go run . print \
  --providers=ingress-nginx \
  --implementations=kgateway \
  --input-file ./pkg/i2gw/implementations/kgateway/testing/testdata/input.yaml
```

The command should generate Gateway API resources and the following Kgateway resources:

### TrafficPolicy

Each TrafficPolicy is attached to routes/backends via:

- targetRefs (when the policy applies to the entire route), or
- extensionRef backend filters (when the policy applies to specific backends).

## Supported Annotations

### Implementation Selection

`ingress2gateway.kubernetes.io/implementation: kgateway`: Tells the ingress-nginx provider to target the Kgateway implementation. This overrides the default GatewayClass name used by the provider.

### Buffer / Body Size

`nginx.ingress.kubernetes.io/client-body-buffer-size`: Max size of the request body buffered in memory. When set, projected into a Kgateway TrafficPolicy.spec.buffer.maxRequestSize.

`nginx.ingress.kubernetes.io/proxy-body-size`: Max allowed request body size (more strict than the client buffer). When present, this takes precedence over client-body-buffer-size.

### CORS

`nginx.ingress.kubernetes.io/enable-cors`: Enables CORS support when set to "true".

`nginx.ingress.kubernetes.io/cors-allow-origin`: Comma-separated list of allowed origins. Projected into Kgateway TrafficPolicy.spec.cors.allowOrigins.

### Rate Limiting

`nginx.ingress.kubernetes.io/limit-rps`: Requests per second limit. Takes precedence over limit-rpm.

`nginx.ingress.kubernetes.io/limit-rpm`: Requests per minute limit (used when RPS is not set).

`nginx.ingress.kubernetes.io/limit-burst-multiplier`: Multiplier used to compute burst capacity for the token bucket.
