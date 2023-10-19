# Ingress to Gateway

Ingress2gateway helps translate Ingress resources to Gateway API resources,
specifically HTTPRoutes. Ingress2gateway is managed by the [Gateway
API](https://gateway-api.sigs.k8s.io/) SIG-Network subproject.

## Scope

Ingress2gateway is primarily focused on translating Ingress and provider-specific
resources(CRDs) to Gateway API resources. Widely used provider-specific annotations
and/or CRDs _may_ still not be supported. Please refer to
[supported providers](#current-supported-providers) for the current supported
providers and their documentation. Contributions for provider-specific
annotations and/or CRDs support are mostly welcomed as long as they can be
translated to [Gateway API](https://gateway-api.sigs.k8s.io/) directly.

Note: Ingress2gateway is not intended to copy annotations from Ingress to Gateway API.

## Installation

If you have a Go development environment locally, you can install ingress2gateway with `go install github.com/kubernetes-sigs/ingress2gateway@v0.1.0`

This will put `ingress2gateway` binary in `$(go env GOPATH)/bin`

Alternatively, you can download the binary at the [releases page](https://github.com/kubernetes-sigs/ingress2gateway/releases)

### Build from Source

1. Ensure that your system meets the following requirements:

   - Install Git: Make sure Git is installed on your system to clone the project repository.
   - Install Go: Make sure the go language is installed on your system. You can download it from the official website (https://golang.org/dl/) and follow the installation instructions.

1. Clone the project repository

   ```shell
   git clone https://github.com/kubernetes-sigs/ingress2gateway.git && cd ingress2gateway
   ```

1. Build the project

   ```shell
   make build
   ```

## Usage

Ingress2gateway reads Ingress resources and/or provider-specifc CRDs from a Kubernetes cluster or a file. It will output the equivalent Gateway API resources in a YAML/JSON format
to stdout. To run ingress2gateway with default options simply run:

```shell
./ingress2gateway print
```

This above command will:
1. Read your Kube config file to extract the cluster credentials and the current active namespace.
1. Search for ingresses and provider-specific resources in that namespace.
1. Convert them to Gateway-API resources (Currently only Gateways and HTTPRoutes).

## Options

### `print` command

| Flag           | Default Value           | Required | Description                                                                                                                                                                             |
| -------------- | ----------------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| namespace      |                         | No       | If present, the namespace scope for the invocation                                                                                                                                      |
| all-namespaces | False                   | No       | If present, list the requested object(s) across all namespaces. Namespace in the current context is ignored even if specified with --namespace                                          |
| output         | yaml                    | No       | The output format, either yaml or json                                                                                                                                                  |
| input_file     |                         | No       | Path to the manifest file. When set, the tool will read ingresses from the file instead of reading from the cluster. Supported files are yaml and json                                  |
| providers      | all supported providers | No       | Comma-separated list of providers. If present, the tool will try to convert only resources related to the specified providers. Otherwise it will default to all the supported providers |

## Conversion of Ingress resources to Gateway API

### Processing Order and Conflicts

Ingress resources will be processed with a defined order to ensure deterministic generated Gateway API configuration.
This should also determine precedence order of Ingress resources and routes in case of conflicts.

Ingress resources with the oldest creation timestamp will be sorted first and therefore given precedence.
If creation timestamps are equal, then sorting will be done based on the namespace/name of the resources.
If an Ingress rule conflicts with another (e.g. same path match but different backends) an error will be reported for the one that sorted later.

Since the Ingress v1 spec does not itself have a conflict resolution guide, we have adopted this one.
These rules are similar to the [Gateway API conflict resolution guidelines](https://gateway-api.sigs.k8s.io/concepts/guidelines/#conflicts).

### Ingress resource fields to Gateway API fields

Given a set of Ingress resources, `ingress2gateway` will generate a Gateway with various HTTP and HTTPS Listeners as well as HTTPRoutes that should represent equivalent routing rules.

| Ingress Field                   | Gateway API configuration                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ingressClassName`              | If configured on an Ingress resource, this value will be used as the `gatewayClassName` set on the corresponding generated Gateway. `kubernetes.io/ingress.class` annotation has the same behavior.                                                                                                                                                                                                                                                                                                                                                                                                               |
| `defaultBackend`                | If present, this configuration will generate a Gateway Listener with no `hostname` specified as well as a catchall HTTPRoute that references this listener. The backend specified here will be translated to a HTTPRoute `rules[].backendRefs[]` element.                                                                                                                                                                                                                                                                                                                                                         |
| `tls[].hosts`                   | Each host in an IngressTLS will result in a HTTPS Listener on the generated Gateway with the following: `listeners[].hostname` = host as described, `listeners[].port` = `443`, `listeners[].protocol` = `HTTPS`, `listeners[].tls.mode` = `Terminate`                                                                                                                                                                                                                                                                                                                                                            |
| `tls[].secretName`              | The secret specified here will be referenced in the Gateway HTTPS Listeners mentioned above with the field `listeners[].tls.certificateRefs`. Each Listener for each host in an IngressTLS will get this secret.                                                                                                                                                                                                                                                                                                                                                                                                  |
| `rules[].host`                  | If non-empty, each distinct value for this field in the provided Ingress resources will result in a separate Gateway HTTP Listener with matching `listeners[].hostname`. `listeners[].port` will be set to `80` and `listeners[].protocol` set to `HTTPS`. In addition, Ingress rules with the same hostname will generate HTTPRoute rules in a HTTPRoute with `hostnames` containing it as the single element. If empty, similar to the `defaultBackend`, a Gateway Listener with no hostname configuration will be generated (if it doesn't exist) and routing rules will be generated in a catchall HTTPRoute. |
| `rules[].http.paths[].path`     | This field translates to a HTTPRoute `rules[].matches[].path.value` configuration.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `rules[].http.paths[].pathType` | This field translates to a HTTPRoute `rules[].matches[].path.type` configuration. Ingress `Exact` = HTTPRoute `Exact` match. Ingress `Prefix` = HTTPRoute `PathPrefix` match.                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `rules[].http.paths[].backend`  | The backend specified here will be translated to a HTTPRoute `rules[].backendRefs[]` element.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |

### Provider-Specific Support

Ingress2gateway also supports translating provider-specific resources and ingress annotations to Gateway-API resources.

#### Current supported providers:

- [ingress-nginx](pkg/i2gw/providers/ingressnginx/README.md)
- [kong](pkg/i2gw/providers/kong/README.md)

If your provider, or a specific feature, is not currently supported, please open an issue and describe your use case.

To contribute a new provider support - please read [PROVIDER.md](PROVIDER.md).


## Get Involved

This project will be discussed in the same Slack channel and community meetings
as the rest of the Gateway API subproject. For more information, refer to the
[Gateway API Community](https://gateway-api.sigs.k8s.io/contributing/) page.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of
Conduct](code-of-conduct.md).

[owners]: https://git.k8s.io/community/contributors/guide/owners.md
[Creative Commons 4.0]: https://git.k8s.io/website/LICENSE
