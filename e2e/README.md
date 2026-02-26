# E2E Tests

End-to-end tests for ingress2gateway.

## Requirements

- Go 1.24+
- Docker (for kind cluster)
- `kubectl`
- [kind](https://kind.sigs.k8s.io/) (auto-installed if missing)

## Running the tests

### Using a local kind cluster (recommended)

```bash
make e2e
```

This will automatically create a kind cluster named `i2gw-e2e`, run the tests and clean up at the
end.

### Using an existing cluster

A generic k8s cluster can be used to execute the tests:

```bash
I2GW_KUBECONFIG=/path/to/kubeconfig make e2e
```

>**NOTE:** Do not have `KUBECONFIG` set in your shell when running tests. The e2e tests
>deliberately refuse to run when this var is set to avoid accidental execution against an unrelated
>cluster.

### Customizing test execution

By default, `make e2e` passes the following arguments to `go test`:

```
-race -v -count=1 -timeout=30m
```

Custom arguments can be passed using `I2GW_GO_TEST_ARGS` for things like running a subset of the
tests or changing the timeout:

```bash
I2GW_GO_TEST_ARGS="-race -v -count=1 -timeout=10m -run TestIngressNginx" make e2e
```

>NOTE: When using a `$` in `-run` as part of a regex, the entire string should be single-quoted AND
>the `$` should be Make-escaped as `$$`.

### Environment variables

| Variable | Description |
|----------|-------------|
| `I2GW_KUBECONFIG` | Path to kubeconfig for an existing cluster. If unset, a kind cluster is created. |
| `I2GW_GO_TEST_ARGS` | Custom arguments to pass to `go test`. Default: `-race -v -count=1 -timeout=30m`. |
| `SKIP_CLEANUP` | Set to `1` to skip cleanup of test resources and kind cluster after tests. |

### Cleanup

To manually delete a leftover kind cluster:

```bash
make clean-kind
```

## Writing new tests

### Test organization

Each test combines three dimensions: a **provider** (reads ingresses and converts them to
an intermediate representation), an **emitter** (transforms the IR into Gateway API
resources and optional implementation-specific CRDs) and a gateway **implementation**
(routes traffic based on those resources).

Rather than testing every (provider × emitter × implementation) combination, the tests are
organized into 5 categories that exercise each dimension independently to maximize coverage
while minimizing repetition. The key insight behind this design is that providers and
implementations are decoupled by the Gateway API contract: the intermediate resources
(`HTTPRoute`, `Gateway`, etc.) are identical regardless of which provider produced them.
This means that if Category 1 proves a provider can emit correct Gateway API resources, and
Category 4 proves an implementation can route traffic from those resources, the
provider → implementation combination is transitively covered without a dedicated test.
The only exception is emitter-specific CRDs (Category 3), which are inherently tied to a
particular implementation and therefore require explicit pairing.

#### Category 1 — provider basics (`provider_test.go`)

One test per provider, all using the Istio implementation and the standard emitter. Validates
that each provider's core ingress → Gateway API conversion works.

#### Category 2 — provider features (`provider_<name>_test.go`)

One test per provider-specific annotation or feature using the Istio implementation and the
standard emitter. Only the provider that owns the feature is used. Validates that the feature is
correctly converted to Gateway API resources.

#### Category 3 — emitter features (TODO)

One test per emitter-specific feature using the ingress-nginx provider paired with the
emitter's corresponding implementation (e.g. `envoygateway` emitter with Envoy Gateway).
Validates emitter-specific CRDs like `BackendTrafficPolicy` or `GCPGatewayPolicy`.

Sample future files: `emitter_envoygateway_test.go`, `emitter_kgateway_test.go` etc.

#### Category 4 — implementation smoke tests (`implementation_test.go`)

One test per gateway implementation, all using the ingress-nginx provider and the standard
emitter. Because gateway implementations only consume standard Gateway API resources, a single
well-tested provider (ingress-nginx, already proven by Categories 1 and 2) is enough.
Using the same provider across all implementations keeps fault isolation clean: a failure in
this category can only be caused by the implementation, never the provider.

#### Category 5 — multi-provider (`multiprovider_test.go`)

Tests that combine multiple providers in a single conversion (e.g. ingress-nginx + kong)
using the Istio implementation and the standard emitter. Ensures that multiple providers can be
used together without conflicts.

### Directory structure

```
e2e/
├── framework/       # Shared test infrastructure (no _test.go files)
├── provider/        # Provider deployment helpers — one .go file per provider
├── implementation/  # Gateway implementation deployment helpers — one .go file per implementation
├── helpers.go       # runTestCase wrapper that wires providers + implementations to the framework
├── *_test.go        # Test files, organized by category (see above)
└── README.md
```

### Auto-generated host field

Setting the `Host` field in ingress rules and in verifiers is optional. When omitted, a random
host is generated and used automatically for all ingress objects and verifiers in the test case.
Most test cases likely don't need an explicit `Host` value since the value doesn't matter as long
as the verifier verifies the correct host.

If a specific `Host` value **is** important for a test case, pay attention to duplicate host values
across test cases: while Kubernetes allows defining multiple ingress objects with identical host
values, whether doing so makes sense (or even works) depends on the ingress controller and can
influence test results.
