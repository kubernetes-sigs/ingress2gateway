# Istio Provider

The provider translates Istio API entities: [Gateway](https://istio.io/latest/docs/reference/config/networking/gateway/) and [VirtualService](https://istio.io/latest/docs/reference/config/networking/virtual-service) to the K8S Gateway API: Gateway, HTTPRoute, TLSRoute, TCPRoute and ReferenceGrants.

If certain field of the Istio API entity lacks a direct equivalent in K8S Gateway API, this field is logged and ignored during the translation.

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

#### TLS

The list of fields showing how istio.VirtualService.Tls fields are converted to the TLSRoute equivalents

* match.sniHosts -> TLSRouteSpec.Hostnames
* route []RouteDestination ->  []gw.BackendRef

#### TCP

The list of fields showing how istio.VirtualService.Tlc fields are converted to the TCPRoute equivalents

* route []RouteDestination ->  []gw.BackendRef
