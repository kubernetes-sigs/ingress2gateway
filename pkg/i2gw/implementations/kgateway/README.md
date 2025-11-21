# Kgateway Emitter

The Kgateway Emitter supports generating **Gateway API** and **Kgateway** resources from Ingress manifests using:

- **Provider**: `ingress-nginx`

**Note:** All other providers will be ignored by the emitter.

## Development Workflow

The typical development workflow for adding an Ingress NGINX feature to the Kgateway emitter is:

1. Use [this issue](https://github.com/kubernetes-sigs/ingress2gateway/issues/232) to prioritize the list of Ingress NGINX features unless
   the business provides requirements. When this list is complete, refer to [this doc](https://docs.google.com/document/d/12ejNTb45hASGYvUjd3t9mfNwuMTX3KBM68ALM4jRFBY/edit?usp=sharing) for additional features. **Note:** Several of the festures from the above list have already been implemented, so review the
   current supported features before adding more.
2. If any of the above features cannot map to an existing Kgateway API, create a Kgateway issue, label it with `kind/ingress-nginx`,
   `help wanted`, `priority/high`, etc. and describe what's needed.
3. Extend the ingress-nginx IR (`pkg/i2gw/intermediate/provider_ingressnginx.go`) as needed. Most changes should fall within the Policy IR.
4. Add a feature-specific function to the ingress-nginx provider (`pkg/i2gw/providers/ingressnginx`), e.g. `proxyReadTimeoutFeature()`
   that parses the Ingress NGINX annotation from source Ingresses and records them as generic Policies in the ingress-nginx provider-specific IR.
5. Update the Kgateway Emitter (`pkg/i2gw/implementations/kgateway/emitter.go`) to consume the IR and return Kgateway-specific resources.
6. Follow the **Testing** section to test your changes.
7. Update the list of supported annotations with the feature you added.
8. Submit a PR to merge your changes upstream. [This branch](https://github.com/danehans/ingress2gateway/tree/impl_emitter_nginx_feat) is the **current** upstream, but [k8s-sigs](https://github.com/kubernetes-sigs/ingress2gateway) or [solo](https://github.com/solo-io/ingress2gateway) repos should be used before releasing.

## Testing

Run the tool with the test input manifest:

```bash
go run . print \
  --providers=ingress-nginx \
  --implementations=kgateway \
  --input-file ./pkg/i2gw/implementations/kgateway/testing/testdata/input.yaml
```

The command should generate Gateway API and Kgateway resources.

## Supported Annotations

### Implementation Selection

- `ingress2gateway.kubernetes.io/implementation: kgateway`: Tells the ingress-nginx provider to target the Kgateway implementation.
  This overrides the default GatewayClass name used by the provider.

### Traffic Behavior

- `nginx.ingress.kubernetes.io/client-body-buffer-size`
- `nginx.ingress.kubernetes.io/proxy-body-size`
- `nginx.ingress.kubernetes.io/enable-cors`
- `nginx.ingress.kubernetes.io/cors-allow-origin`
- `nginx.ingress.kubernetes.io/limit-rps`
- `nginx.ingress.kubernetes.io/limit-rpm`
- `nginx.ingress.kubernetes.io/limit-burst-multiplier`
- `nginx.ingress.kubernetes.io/proxy-send-timeout`
- `nginx.ingress.kubernetes.io/proxy-read-timeout`

### Backend Behavior

- `nginx.ingress.kubernetes.io/proxy-connect-timeout`

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

## BackendConfigPolicy Projection

Annotations in the **Backend Behavior** category are converted into
`BackendConfigPolicy` resources.

Currently supported:

- `proxy-connect-timeout`

If multiple Ingresses target the same Service with conflicting values,
the lowest timeout wins and a warning is emitted.

### Summary of Policy Types

| Annotation Type              | Kgateway Resource     |
|------------------------------|-----------------------|
| Request/response behavior    | `TrafficPolicy`       |
| Upstream connection behavior | `BackendConfigPolicy` |

## Limitations

- Only the **ingress-nginx provider** is currently supported by the Kgateway emitter.
- Some NGINX behaviors cannot be reproduced exactly due to Envoy/Kgateway differences.
