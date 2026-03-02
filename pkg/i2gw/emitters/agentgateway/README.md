# agentgateway emitter

This folder contains the `agentgateway` emitter.

## Current behavior (minimal)

- Emits standard Gateway API resources from the shared `EmitterIR`
- Sets `Gateway.spec.gatewayClassName: agentgateway` on all emitted Gateways

This is enough to let users target the Agent Gateway implementation via:

```bash
go run . print --providers=... --emitter=agentgateway --input-file <file>
```

## Roadmap

As upstream `EmitterIR` grows additional provider-neutral intents (and/or explicit policy IR), this emitter can be extended to emit agentgateway-specific extension resources.
