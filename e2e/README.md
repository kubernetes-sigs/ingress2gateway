# E2E Tests

End-to-end tests for ingress2gateway.

## Requirements

- Go 1.24+
- Docker (for kind cluster)
- `kubectl`
- [kind](https://kind.sigs.k8s.io/) (auto-installed if missing)

## Running Tests

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

### Running specific tests

A specific subset of the tests can be run by passing a value to `go test -run` using
`I2GW_RUN_TESTS`:

```bash
I2GW_RUN_TESTS="TestIngressNginx" make e2e
```

Standard Go test filtering using regex is also supported:

```bash
I2GW_RUN_TESTS="^TestIngressNginx/to_Istio/foo.*$" make e2e
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `I2GW_KUBECONFIG` | Path to kubeconfig for an existing cluster. If unset, a kind cluster is created. |
| `I2GW_RUN_TESTS` | Regex pattern to filter which tests to run (passed to `go test -run`). |
| `SKIP_CLEANUP` | Set to `1` to skip cleanup of test resources and kind cluster after tests. |

## Cleanup

To manually delete a leftover kind cluster:

```bash
make clean-kind
```
