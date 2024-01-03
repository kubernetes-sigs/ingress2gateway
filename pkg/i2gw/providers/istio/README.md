# Istio Provider

The provider translates Istio API entities: [Gateway](https://istio.io/latest/docs/reference/config/networking/gateway/) and [VirtualService](https://istio.io/latest/docs/reference/config/networking/virtual-service) to the K8S Gateway API: Gateway, HTTPRoute, TLSRoute, TCPRoute and ReferenceGrants.

The API translator converts the API fields that have a direct equivalent in the K8S Gateway API. If a certain field of the Istio API cannot be translated directly, this field would be logged and ignored during the translation. It's up to the user to handle such cases accordingly to their needs.

## Examples

You can find the examples demonstrating how the resources are translated within the [fixtures](./fixtures/) directory.

There are examples for:

* Istio Gateway -> K8S API Gateway
* VirtualService -> HTTPRoute
* VirtualService -> TLSRoute
* VirtualService -> TCPRoute
* Creation of the K8S API ReferenceGrants for cross namespace references

## Conversion of Ingress resources to Gateway API

### Generated ReferenceGrants and xRoute.parentRefs

Translator verifies if the xRoute can be connected to the Gateway and if parentRefs to it should be generated.

It considers the following fields:

`parentRefs` would be generated if:

1. VirtualService can be exported to the Gateway's namespaces, values from `virtualService.Spec.ExportTo`
2. There's an overlap between Gateway's `Server.Hosts` and `virtualService.Spec.Hosts`

If Gateway and VirtualService are in the different namespaces, then a `ReferenceGrant` would be created to allow translated xRoute to reference translated Gateway.

### Istio Gateway

K8S API Gateway Listener is generated for each host of each server of istio gateway.Spec.Server.

Listener names are generated in the following format: `$PROTOCOL_NAME-protocol-$NAMESPACE-ns-$HOSTNAME"`. The format is chosen to ensure API compliance where all listener names MUST be unique within the Gateway.

#### Protocols

Istio supported protocols -> K8S Gateway Listener protocols

* HTTP|HTTPS|TCP|TLS - converted as is
* HTTP2|GRPC -> if `tls` is set then HTTPS else HTTP
* MONGO -> TCP

#### TLS

Modes translation:

* PASSTHROUGH and AUTO_PASSTHROUGH -> gw.TLSModePassthrough
* SIMPLE and MUTUAL -> gw.TLSModeTerminate
* other istio tls modes are not translated

### Istio VirtualService

#### HTTP

The list of fields showing how istio.VirtualService.Http fields are converted to the HTTPRoute equivalents

* match []HTTPMatchRequest -> []gw.HTTPRouteMatch
* route []HTTPRouteDestination -> []gw.HTTPBackendRef
* redirect HTTPRedirect -> gw.HTTPRequestRedirectFilter
* rewrite HTTPRewrite -> gw.HTTPURLRewriteFilter
* timeout Duration -> gw.HTTPRouteTimeouts.Request
* mirror and mirrors -> []gw.HTTPRequestMirrorFilters
* headers.request -> requestHeaderModifier gw.HTTPHeaderFilter
* headers.response -> responseHeaderModifier gw.HTTPHeaderFilter

##### rewrite HTTPRewrite translation

In istio, the rewrite logic depends on the match URI parameters:
 * for prefix match, istio rewrites matched prefix to the given value.
 * for exact match and for regex match, istio rewrites full URI path to the given value.

Also, in K8S Gateway API only 1 HTTPRouteFilterURLRewrite is allowed per HTTPRouteRule
https://github.com/kubernetes-sigs/gateway-api/blob/0ad0daffe8d47f97a293b2a947bb3b2ee658e967/apis/v1/httproute_types.go#L228

To take this all into consideration, translator aggregates prefix matches vs non-prefix matches for the istio virtualservice.HTTPRoute.
And generates max 2 HTTPRoutes (one with prefix matches and ReplacePrefixMatch filter and the other if non-prefix matches and ReplaceFullPath filter).
If any of the match group is empty, the corresponding HTTPRoute won't be generated.
If all URI matches are empty, there would be HTTPRoute with HTTPRouteFilterURLRewrite of ReplaceFullPath type.

#### TLS

The list of fields showing how istio.VirtualService.Tls fields are converted to the TLSRoute equivalents

* match.sniHosts -> TLSRouteSpec.Hostnames
* route []RouteDestination ->  []gw.BackendRef

#### TCP

The list of fields showing how istio.VirtualService.Tlc fields are converted to the TCPRoute equivalents

* route []RouteDestination ->  []gw.BackendRef
