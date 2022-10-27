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

## Usage

This project reads Ingress resources from a Kubernetes cluster based on your
current Kube Config. It will output YAML for equivalent Gateway API resources
to stdout. Until this project is released, the best way to use this is to run
the following within the repo:

```
go run .
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
| `ingressClassName` | |
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

* kubernetes.io/ingress.class

#### ingress-nginx:

* nginx.ingress.kubernetes.io/canary
* nginx.ingress.kubernetes.io/canary-by-header
* nginx.ingress.kubernetes.io/canary-by-header-value
* nginx.ingress.kubernetes.io/canary-by-header-pattern
* nginx.ingress.kubernetes.io/canary-weight
* nginx.ingress.kubernetes.io/canary-weight-total

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
