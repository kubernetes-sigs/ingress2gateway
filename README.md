# Ingress to Gateway

This project helps translate Ingress resources to Gateway API resources,
specifically HTTPRoutes. This project is managed by the [Gateway
API](https://gateway-api.sigs.k8s.io/) SIG-Network subproject.

## Status

This project is early in the development phase and is still experimental in
nature. Both bugs and breaking changes are likely.

## Scope

This project is primarily focused on translating Ingress resources to Gateway
API resources. Some widely used annotations and/or CRDs _may_ be supported, as
long as they can be translated to Gateway API directly. This project is not
intended to copy annotations from Ingress to Gateway API.

## Install

This project reads Ingress resources from a Kubernetes cluster based on your
current Kube Config. It will output YAML for equivalent Gateway API resources
to stdout. Until this project is released, the best way to use this is to run
the following within the repo:

```shell
curl -fsSL https://raw.githubusercontent.com/kubernetes-sigs/ingress2gateway/main/tools/hack/install.sh | bash 
```

## QuickStart

```shell
curl -fsSL https://raw.githubusercontent.com/kubernetes-sigs/ingress2gateway/main/examples/demo.yaml | i2gw translate --mode=local -f -
```

## Development

``` shell
make help
```

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

| Ingress Field | Gateway API configuration |
|---------------|---------------------------|
| `ingressClassName` | If configured on an Ingress resource, this value will be used as the `gatewayClassName` set on the corresponding generated Gateway. |
| `defaultBackend` | If present, this configuration will generate a Gateway Listener with no `hostname` specified as well as a catchall HTTPRoute that references this listener. The backend specified here will be translated to a HTTPRoute `rules[].backendRefs[]` element. |
| `tls[].hosts` | Each host in an IngressTLS will result in a HTTPS Listener on the generated Gateway with the following: `listeners[].hostname` = host as described, `listeners[].port` = `443`, `listeners[].protocol` = `HTTPS`, `listeners[].tls.mode` = `Terminate` |
| `tls[].secretName` | The secret specified here will be referenced in the Gateway HTTPS Listeners mentioned above with the field `listeners[].tls.certificateRefs`. Each Listener for each host in an IngressTLS will get this secret. |
| `rules[].host` | If non-empty, each distinct value for this field in the provided Ingress resources will result in a separate Gateway HTTP Listener with matching `listeners[].hostname`. `listeners[].port` will be set to `80` and `listeners[].protocol` set to `HTTPS`. In addition, Ingress rules with the same hostname will generate HTTPRoute rules in a HTTPRoute with `hostnames` containing it as the single element. If empty, similar to the `defaultBackend`, a Gateway Listener with no hostname configuration will be generated (if it doesn't exist) and routing rules will be generated in a catchall HTTPRoute. |
| `rules[].http.paths[].path` | This field translates to a HTTPRoute `rules[].matches[].path.value` configuration. |
| `rules[].http.paths[].pathType` | This field translates to a HTTPRoute `rules[].matches[].path.type` configuration. Ingress `Exact` = HTTPRoute `Exact` match. Ingress `Prefix` = HTTPRoute `PathPrefix` match. |
| `rules[].http.paths[].backend` | The backend specified here will be translated to a HTTPRoute `rules[].backendRefs[]` element. |

### Implementation-Specific Annotations

Although most annotations are ignored, this project includes experimental
support for the following annotations:

* `kubernetes.io/ingress.class`: Same behavior as the `ingressClassName` field above, if specified this value will be used as the `gatewayClassName` set on the corresponding generated Gateway.

#### ingress-nginx:

* `nginx.ingress.kubernetes.io/canary`: If set to `true` will enable weighting backends.
* `nginx.ingress.kubernetes.io/canary-by-header`: If specified, the value of this annotation is the header name that will be added as a HTTPHeaderMatch for the routes generated from this Ingress. If not specified, no HTTPHeaderMatch will be generated.
* `nginx.ingress.kubernetes.io/canary-by-header-value`: If specified, the value of this annotation is the header value to perform an `HeaderMatchExact` match on in the generated HTTPHeaderMatch.
* `nginx.ingress.kubernetes.io/canary-by-header-pattern`: If specified, this is the  pattern to match against for the HTTPHeaderMatch, which will be of type `HeaderMatchRegularExpression`.
* `nginx.ingress.kubernetes.io/canary-weight`: If specified and non-zero, this value will be applied as the weight of the backends for the routes generated from this Ingress resource.
* `nginx.ingress.kubernetes.io/canary-weight-total`

If you are reliant on any annotations not listed above, you'll need to manually
find a Gateway API equivalent.

## Get Involved

This project will be discussed in the same Slack channel and community meetings
as the rest of the Gateway API subproject. For more information, refer to the
[Gateway API Community](https://gateway-api.sigs.k8s.io/contributing/) page.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of
Conduct](code-of-conduct.md).

[owners]: https://git.k8s.io/community/contributors/guide/owners.md
[Creative Commons 4.0]: https://git.k8s.io/website/LICENSE
