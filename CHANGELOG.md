# Changelog

## Table of Contents

- [v0.2.0-rc1](#v020-rc1)
- [v0.1.0](#v010)
- [v0.1.0-rc1](#v010-rc1)

## v0.2.0-rc1

### Notable changes since v0.1.0
#### Providers storage
Providers now fetch resources and store them in their local storage.
This expands to ingress fetching. It is no longer happening on i2gw package and moved to be fetched at the provider level.

#### New Providers
Istio and APISIX support has been added. 
To check what features are currently supported please visit [Istio](https://github.com/kubernetes-sigs/ingress2gateway/blob/v0.2.0-rc1/pkg/i2gw/providers/istio/README.md) and [APIXIS](https://github.com/kubernetes-sigs/ingress2gateway/blob/v0.2.0-rc1/pkg/i2gw/providers/apisix/README.md).

### Feature

- Add support for Istio API conversion to K8S Gateway API (#111, @dpasiukevich)
- Kong supports `ImplementationSpecific` as `PathType` and converts it into `RegularExpression.` (#89, @mlavacca)
- Move ingress fetching logic to be isolated, per provider (#116, @LiorLieberman)
- New Apache APISIX provider. (#108, @pottekkat)
- Print generated GatewayClasses, TLSRoutes, TCPRoutes and ReferenceGrants in addition to Gateways and HTTPRoutes (#110, @dpasiukevich)
- The `HTTPRoutes` are named with the following pattern: <ingress-name>-<name-from-host>. (#79, @mlavacca)
- [Istio provider] set up code for reading istio custom resources (#99, @dpasiukevich)

### Bug or Regression

- Duplicate `BackendRefs` are removed from the `HTTPRoute` rules. (#104, @pottekkat)

## Dependencies

- Gateway API has been bumped to v1.0.0. (#98, @mlavacca)

## v0.1.0
The first official release of ingress2gateway.

### Notable changes since v0.1.0-rc1

### Feature

- [Kong Provider] Add support for converting the `konghq.com/plugins` ingress annotation to a list of `ExtensionRef` HTTPRoute filters. (#72, @mlavacca)

## v0.1.0-rc1
initial release candidate. 
