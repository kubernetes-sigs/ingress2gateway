# Agentgateway emitter E2E test suite

This package contains an end-to-end (E2E) test suite that validates **Ingress → ingress-nginx routing** and
**Ingress2Gateway-generated Gateway API routing via agentgateway** against real workloads in a **kind** cluster.

It is designed to be easy to extend by adding more test case YAMLs.

---

## What the suite does

Once per test run:

1. Uses an existing **kind** cluster if it already exists; otherwise creates one.
2. Installs **MetalLB** so `Service type=LoadBalancer` gets an external IP in kind.
3. Installs **Gateway API CRDs**.
4. Installs **ingress-nginx**.
5. Installs **kgateway eith the agentgateway data plane** (version inferred from `go.mod` unless overridden).
6. Deploys a shared **echo backend** (`echo-basic`).

**Note:** HTTP requests are made directly from test code using Gateway API conformance utilities (`sigs.k8s.io/gateway-api/conformance/utils/http` and `sigs.k8s.io/gateway-api/conformance/utils/tls`), not from an in-cluster pod.

For each test case (each input YAML under `testdata/input/`):

1. Applies the **input** Ingress YAML.
   - Waits for the Ingress to get an external IP and verifies connectivity (HTTP request → Ingress → backend).
   - Applies the **output** Gateway API YAML.
   - Waits for GatewayClass/Gateway/HTTPRoute readiness conditions.
   - Fetches the Gateway's external IP and verifies connectivity (HTTP request → Gateway → backend).
   - Cleans up only the per-test input/output resources (keeps echo backend).

---

## Directory layout

```text
test/e2e/emitters/agentgateway
├── e2e_test.go
...
└── testdata
    ├── input
    │   ├── basic.yaml
    │   └── <future>.yaml
    └── output
        ├── basic.yaml
        └── <future>.yaml
```

**Important:** each input file must have a matching output file with the same basename.

Example:

- `testdata/input/basic.yaml`
- `testdata/output/basic.yaml`

---

## Prerequisites

You must have the following on your PATH:

- `docker`
- `kind`
- `kubectl`
- `helm`
- `go`

The suite uses a kind cluster per run and **deletes it by default**.
Set `KEEP_KIND_CLUSTER=true` to keep the cluster after the test run.

If the cluster named by `KIND_CLUSTER_NAME` already exists when the test suite starts, it will be reused.
By default it will still be deleted at the end of the run unless `KEEP_KIND_CLUSTER=true`.

---

## Running the tests

From the repo root using the `make` target:

```bash
make test-e2e
```

From the repo root using `go test`:

```bash
go test -v ./test/e2e/emitters/agentgateway/...
```

To run a single test case (subtest name matches the input filename):

```bash
go test -v ./test/e2e/emitters/agentgateway -run 'TestBasic'
```

---

### Cluster control

| Variable | Default | Description |
|---|---:|---|
| `KIND_CLUSTER_NAME` | `i2g-e2e` | kind cluster name |
| `KEEP_KIND_CLUSTER` | `true` | if `true` or unset, leaves the cluster running after the test run; set to `false` to delete |
| `KIND_NODE_IMAGE` | (kind default) | override kind node image, e.g. `kindest/node:v1.33.1` |

Example:

```bash
KEEP_KIND_CLUSTER=false go test ./test/e2e/emitters/agentgateway -v -run TestIngress2GatewayE2E
```

### Component versions

| Variable | Default | Description |
|---|---:|---|
| `INGRESS_NGINX_VERSION` | `v1.14.1` | used in URL `controller-${VERSION}` |
| `GATEWAY_API_VERSION` | `v1.4.0` | applies `experimental-install.yaml` |
| `METALLB_VERSION` | `v0.15.3` | applies `metallb-native.yaml` |
| `AGENTGATEWAY_VERSION` | (derived) | overrides Helm chart version. If unset, version is derived from `go.mod` and normalized for agentgateway release naming |

### Images

| Variable | Default | Description |
|---|---:|---|
| `ECHO_IMAGE` | `gcr.io/k8s-staging-gateway-api/echo-basic:...` | backend echo server image |
| `GRPC_ECHO_IMAGE` | `gcr.io/k8s-staging-gateway-api/echo-basic:v20240412-v1.0.0-394-g40c666fd` | gRPC backend image for `TestBackendProtocol` |

**Notes:**
- HTTP requests are made directly from test code, so no curl client pod image is needed.
- The shared `echo-backend` stays in HTTP mode for general test coverage.
- `TestBackendProtocol` creates a dedicated `echo-backend-grpc` deployment/service with `GRPC_ECHO_SERVER=true` so gRPC validation does not alter other HTTP-focused cases.

Example override:

```bash
AGENTGATEWAY_VERSION=v2.2.0-beta.1 \
  go test ./test/e2e/emitters/agentgateway -v -run TestIngress2GatewayE2E
```

---

## How agentgateway version selection works

If `AGENTGATEWAY_VERSION` is **set**, the suite uses it directly for Helm `--version`.

If it is **unset**, the suite:

1. Reads `github.com/agentgateway-dev/agentgateway/v2 <version>` from `go.mod`
2. Strips the Go pseudo-version suffix like `.<timestamp>-<sha>` (e.g. `.20251203210329-f0eb663ac5bd`)
3. For agentgateway beta tags, also strips the trailing `.0` (e.g. `v2.2.0-beta.1.0` → `v2.2.0-beta.1`)

This ensures Helm can pull valid chart tags from the OCI registry even when `go.mod` contains a pseudo-version.

---

## Adding a new test case

1. Create a new input YAML in:

    ```text
    test/e2e/emitters/agentgateway/testdata/input/<case>.yaml
    ```

2. Create the matching output YAML in:

    ```text
    test/e2e/emitters/agentgateway/testdata/output/<case>.yaml
    ```

3. Run:

    ```bash
    go test ./test/e2e/emitters/agentgateway -v -run TestIngress2GatewayE2E/<case>
    ```

### Guidelines for input YAML

Your **input** file should include an `Ingress` that routes to the shared backend service:

- Service: `echo-backend`
- Service port: `8080`

The test suite will:

- wait for `.status.loadBalancer.ingress[]` on the Ingress
- make an HTTP request to the Ingress IP using the first `spec.rules[].host` it finds (fallback: `demo.localdev.me`)

So your Ingress should include a host if you want a deterministic Host header.

### Guidelines for output YAML

Your **output** file should include the Gateway API resources produced by `ingress2gateway` for that input, typically:

- `Gateway`
- `HTTPRoute`
- Any agentgateway CRDs required for the translation (TrafficPolicy, etc.)

The test suite will:

- wait for `GatewayClass Accepted=True` (for the `GatewayClass/agentgateway` object present)
- wait for `Gateway Accepted=True` and `Programmed=True`
- wait for `HTTPRoute` parent conditions `Accepted=True` and `ResolvedRefs=True`
- make an HTTP request to the Gateway external address using the first `HTTPRoute.spec.hostnames[]` it finds (fallback: the Ingress host)

---

## Debugging failures

Inspect:

```bash
kubectl --context kind-i2g-e2e get pods -A
kubectl --context kind-i2g-e2e -n ingress-nginx get svc,deploy,pods
kubectl --context kind-i2g-e2e -n agentgateway-system get all
kubectl --context kind-i2g-e2e get gatewayclass,gateway,httproute -A
```

### HTTP debug output

If connectivity never reaches HTTP 200, the suite logs detailed HTTP request/response information for debugging.

### Common issues

- **503 from ingress-nginx** right after apply: expected transient behavior; suite retries (`requireHTTP200Eventually`).
- **No external IP on Ingress/Gateway**: MetalLB not ready or IP pool misconfigured.
- **Gateway not programmed**: agentgateway pods not ready, bad config, or missing/invalid refs in output YAML.

---

## Cleanup behavior

- The kind cluster is **kept automatically** unless `KEEP_KIND_CLUSTER=false`.
- Per-test resources from `testdata/input/<case>.yaml` and `testdata/output/<case>.yaml` are deleted after each subtest.
- Shared resources (echo backend) remain for the whole suite run.

---

## Recommended workflow

1. Start with a new input Ingress YAML.
2. Confirm ingress-nginx connectivity passes.
3. Generate output YAML using ingress2gateway.
4. Confirm Gateway API agentgateway connectivity passes.
5. Commit both input and output files together.
