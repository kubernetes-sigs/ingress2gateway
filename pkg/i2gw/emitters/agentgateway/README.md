# agentgateway emitter

This folder contains the `agentgateway` emitter.

## Current behavior

- Emits the shared Gateway API objects produced by `EmitterIR`.
- Sets `Gateway.spec.gatewayClassName: agentgateway` on every Gateway so clients target the agentgateway controller.
- Builds `AgentgatewayPolicy` resources per HTTPRoute rule, converts buffer-related `BodySize` intent into `Frontend.HTTP.maxBufferSize`, and emits those policies as Gateway extensions (side-loaded via `utils.ToGatewayResources`).
- Pushes notifications when buffer size is coerced or max vs buffer precedence is resolved, so callers can see what features were applied.

Run it with:

```bash
go run . print --providers=<provider> --emitter=agentgateway --input-file <file>
```

## Testing once Go and a Kubernetes cluster are available

```bash
go test ./pkg/i2gw/emitters/agentgateway
```

Until upstream provides richer Policy IR, this emitter intentionally keeps policy handling minimal. Future work can extend the builder (`builder.go`) to map additional ingress-nginx policies into `AgentgatewayPolicy.Spec`, mirroring what the downstream emitter does.