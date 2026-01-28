# E2E Tests

End-to-end tests for ingress2gateway.

## Requirements

- Go 1.24+
- Docker (for kind cluster)
- `kubectl`
- [kind](https://kind.sigs.k8s.io/) (auto-installed if missing)

## Running tests

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

## Environment variables

| Variable | Description |
|----------|-------------|
| `I2GW_KUBECONFIG` | Path to kubeconfig for an existing cluster. If unset, a kind cluster is created. |
| `I2GW_GO_TEST_ARGS` | Custom arguments to pass to `go test`. Default: `-race -v -count=1 -timeout=30m`. |
| `SKIP_CLEANUP` | Set to `1` to skip cleanup of test resources and kind cluster after tests. |

## Cleanup

To manually delete a leftover kind cluster:

```bash
make clean-kind
```
