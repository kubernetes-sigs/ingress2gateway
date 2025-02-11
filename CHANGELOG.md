# Changelog

## Table of Contents

- [v0.4.0](#v040)
- [v0.4.0-rc1](#v040-rc1)
- [v0.3.0](#v030)
- [v0.3.0-rc1](#v030-rc1)
- [v0.2.0](#v020)
- [v0.2.0-rc1](#v020-rc1)
- [v0.1.0](#v010)
- [v0.1.0-rc1](#v010-rc1)

## v0.4.0
No changes since [v0.4.0-rc1](#v040-rc1)
## Changes by Kind

### Other (Cleanup or Flake)

Fix goreleaser (#207, @subrajeet-maharana)

## v0.4.0-rc1

## Major Themes

### New Cilium support
Added support for translating Cilium Ingress to Gateway API 

### Enhanced GKE support
GKE translation now supports translating Cloud Armor, Custom HealthChecks, and SSL Policies to their GKE Gateway equivalents (by @sawsa307)

## Changes by Kind

### Feature

- Added support for Cilium Ingress to Gateway API (#199, @xtineskim)
- Added notifications for OpenAPI3 (#178, @Devaansh-Kumar)
- Gateways translated via ingress2gateway will be attached with a new annotation `gateway.networking.k8s.io/generator` to track resources generated with ingress2gateway tool and its version. (#187, @sawsa307)
- Added support for translating Cloud Armor security policy on GKE Ingress to GCPBackendPolicy on GKE Gateway. (#191, @sawsa307)
- Added support for translating Custom Health Check on GKE Ingress to HealthCheckPolicy on GKE Gateway. (#194, @sawsa307)
- Added support for translating SSL Policy on GKE Ingress to GCPGatewayPolicy on GKE Gateway. (#195, @sawsa307)

## v0.3.0

## Major Themes

### New Notifications Package

new notifications package to provide mechanism for providers to display useful information generated during conversion process (#160, @Devaansh-Kumar)

## Changes by Kind

### Feature

- Added notifications for Apisix (#176, @Devaansh-Kumar)
- Added notifications for Istio provider (#171, @Devaansh-Kumar)
- Added notifications for Kong (#173, @Devaansh-Kumar)
- Added notifications for ingress-nginx (#177, @Devaansh-Kumar)
- GCE now will display useful information generated during conversion process via the new notification package. (#169, @sawsa307)

### Bug or Regression

- Fix nginx canary annotation conversion (#182, @levikobi)
- Fixed an issue that when the ingress class annotation is not specified on a GKE Ingress, the translation would result in a Gateway without name. (#167, @sawsa307)

## v0.3.0-rc1

## Changes by Kind

### Feature

- Add a `--kubeconfig` flag to specify kubeconfig file location (#133, @YTGhost)
- Added support for GCE provider. (#148, @sawsa307)
- Bump `ReferenceGrant` to `v1beta1` (#142, @YTGhost)
- Deprecate `i2gw.InputResources` and remove input resources from `ToGatewayAPI` function (#141, @YTGhost)
- New support for OpenAPI Provider (#157, @guicassolato)
- Providers flag is now required (#159, @LiorLieberman)
- The Kong `TCPIngress` resources are properly translated into `Gateway`s, `TCPRoute`s, and `TLSRoute`s (#86, @mlavacca)

### Bug or Regression

- Add translation when canary-weight is set to 0 (#137, @MregXN)
- Fix errors when CRDs are not installed in the cluster (#153, @LiorLieberman)
- Fixed ingress-nginx conversion tests (#139, @LiorLieberman)
- Improve error handling for Kong and Ingress Nginx providers, also prevents the tool from crashing in case no `pathType` is specified (#152, @levikobi)
- Allow & handle wildcard hosts in Istio VirtualServices (#155, @zirain)

### Other (Cleanup or Flake)

- The `--input_file` flag has been renamed `--input-file`. (#156, @mlavacca)

## v0.2.0

### Major Themes

#### Providers storage

Providers now fetch resources and store them in their local storage.
This expands to ingress fetching. It is no longer happening on i2gw package and moved to be fetched at the provider level.

#### New Providers

Istio and APISIX support has been added.
To check what features are currently supported please visit [Istio](https://github.com/kubernetes-sigs/ingress2gateway/blob/v0.2.0/pkg/i2gw/providers/istio/README.md) and [APIXIS](https://github.com/kubernetes-sigs/ingress2gateway/blob/v0.2.0/pkg/i2gw/providers/apisix/README.md).

### Feature

- Add support for Istio API conversion to K8S Gateway API (#111, @dpasiukevich)
- Kong supports `ImplementationSpecific` as `PathType` and converts it into `RegularExpression.` (#89, @mlavacca)
- Move ingress fetching logic to be isolated, per provider (#116, @LiorLieberman)
- New Apache APISIX provider. (#108, @pottekkat)
- Print generated GatewayClasses, TLSRoutes, TCPRoutes and ReferenceGrants in addition to Gateways and HTTPRoutes (#110, @dpasiukevich)
- The `HTTPRoutes` are named with the following pattern: <ingress-name>-<name-from-host>. (#79, @mlavacca)
- [Istio provider] set up code for reading istio custom resources (#99, @dpasiukevich)

### Bug or Regression

- Skip k8s client creation when reading local file. (#128, @dpasiukevich)
- Duplicate `BackendRefs` are removed from the `HTTPRoute` rules. (#104, @pottekkat)

## Dependencies

- Gateway API has been bumped to v1.0.0. (#98, @mlavacca)

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
