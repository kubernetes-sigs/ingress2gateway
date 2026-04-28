# Changelog

## Table of Contents

- [v1.1.0](#v110)
- [v1.0.0](#v100)
- [v1.0.0-rc1](#v100-rc1)
- [v0.5.0](#v050)
- [v0.5.0-rc1](#v050-rc1)
- [v0.4.0](#v040)
- [v0.4.0-rc1](#v040-rc1)
- [v0.3.0](#v030)
- [v0.3.0-rc1](#v030-rc1)
- [v0.2.0](#v020)
- [v0.2.0-rc1](#v020-rc1)
- [v0.1.0](#v010)
- [v0.1.0-rc1](#v010-rc1)

## v1.1.0

## Changes by Kind

### Feature

- Added Traefik provider support with translation for Traefik Ingress resources and some annotations. (#400, @FarnazBGH)
- GCE: Added Cloud CDN support via GCPHTTPFilter. (#403, @chakravardhan)
- ingress-nginx: Added support for `from-to-www-redirect`. (#337, @chakravardhan)
- ingress-nginx: Added support for `app-root`. (#405, @jgreeer)
- ingress-nginx: Added support for `ssl-passthrough` using standard-channel TLSRoute resources. (#402, @jgreeer)

### Other (Cleanup or Flake)

- Moved HTTPRoute rule naming from common provider conversion into emitters. (#397, @jgreeer)
- Updated TLSRoute usage to the standard Gateway API channel. (#404, @jgreeer)
- Fixed GoReleaser configuration. (#395, @ponkio-o)
- Updated GitHub Actions versions in build and release workflows. (#406, @sturman)
- Docs: README correction. (#398, @ziyi-xuu)

## v1.0.0

## Major Themes

### Emitters Framework

New pluggable emitter architecture enabling output to vendor-specific Gateway API
extensions. Supported emitters: standard (vanilla Gateway API), Agentgateway, Envoy Gateway, and Kgateway.
(#265, #273, #305, #320)

### Extensive ingress-nginx Annotation Support

Significantly expanded ingress-nginx annotation coverage, adding translation for
header manipulation, GRPC, canary routing, path rewriting, timeouts, SSL and permanent/temporal redirects,
CORS, regex path matching, backend TLS, buffer sizing, IP access control.
A new annotation tracking system also
reports which annotations were parsed, unsupported, or unrecognized.

### E2E Test Framework

Comprehensive end-to-end test suite built in pure Go with real cluster testing
across Ingress NGINX and Envoy Gateway providers. Covers TLS termination, SSL
redirect, canary routing, CORS, and timeouts. (#294, #330, #353, #366, #372)

## Changes by Kind

### Feature

- Emitters framework: pluggable emitter architecture separating providers (Ingress → IR) from emitters (IR → Gateway API resources). Includes standard, Agentgateway, Envoy Gateway, kgateway, and GCE emitters. (#265, #273, #305, #320, #336, #388, @Stevenjin8, @kkk777-7, @puertomontt, @chakravardhan, @markuskobler)
- Route rule name support for xPolicy CRD attachment (#298, @kkk777-7)
- ingress-nginx: header manipulation (`upstream-vhost`, `x-forwarded-prefix`, `connection-proxy-header`) (#283, @eladmotola)
- ingress-nginx: GRPC support annotation  (#286, @eladmotola)
- ingress-nginx: extended canary support with `canary-by-header`, `canary-by-header-value`, and cookie-based routing (#287, #365, #374, @jgreeer, @Stevenjin8)
- ingress-nginx: `rewrite-target` annotation for path rewriting (#288, @Stevenjin8)
- ingress-nginx: timeout annotations (`proxy-connect-timeout`, `proxy-send-timeout`, `proxy-read-timeout`) (#289, #376, #377, @Stevenjin8)
- ingress-nginx: `permanent-redirect` and `temporal-redirect` annotations with configurable status codes (#299, @jgreeer)
- ingress-nginx: full CORS configuration (`allow-origin`, `allow-methods`, `allow-headers`, `allow-credentials`, `expose-headers`, `max-age`). No longer requires `--allow-experimental-gw-api` flag. (#303, #371, @chakravardhan, @kkk777-7)
- ingress-nginx: `use-regex` annotation with `implementationSpecific` path matching (#307, #344, @chakravardhan, @Stevenjin8)
- ingress-nginx: Backend TLS via `proxy-ssl-verify` and `proxy-ssl-secret`, translated to BackendTLSPolicy (#308, @rajashish)
- ingress-nginx: `proxy-body-size` and `client-body-buffer-size` buffer annotations (#305, #375, @kkk777-7, @Stevenjin8)
- ingress-nginx: `whitelist-source-range` and `denylist-source-range` IP access control (#345, @kkk777-7)
- ingress-nginx: `ssl-redirect` annotation with per-route evaluation matching ingress-nginx per-location semantics (#290, #385, @Stevenjin8)
- ingress-nginx: trailing slash redirects (#385, @Stevenjin8)
- ingress-nginx: annotation tracking with notifications for unsupported/unparsed annotations (#359, #361, #370, @Stevenjin8, @kkk777-7)
- Read resources from multiple input files and directories via `--input-file` (#258, #357, @carmal891, @johananl)
- Refactored notification system to provider- and emitter-scoped reports (#360, #384, @johananl, @Stevenjin8)
- E2E test suite with real cluster testing across Ingress NGINX and Envoy Gateway (#294, #330, #351, #353, #366, #372, @johananl, @Stevenjin8, @kkk777-7)

### Bug or Regression

- Fix data race in NotificationAggregator (#292, @johananl)
- Fix setting proper secret group and kind in TLS certificateRefs (#302, @cnvergence)
- Fix panic on nil `ingress.rules.http` (#335, @Stevenjin8)
- Fix deduplicate TLS CertificateRefs in gateway listeners (#378, @Stevenjin8)

### Other (Cleanup or Flake)

- Upgraded Gateway API to v1.5 (#367, @Stevenjin8)
- Migrate to golangci-lint v2 (#323, @kkk777-7)
- Bump Kong chart to v3.0.2 (#349, @johananl)
- Docs: Providers vs Emitters architecture description (#369, @markuskobler)
- Update main and ingress-nginx README (#390, @Stevenjin8)

## v1.0.0-rc1

## Major Themes

### Emitters Framework

New pluggable emitter architecture enabling output to vendor-specific Gateway API
extensions. Providers now produce an intermediate representation (IR) that is
transformed by emitters into Gateway API resources with optional vendor-specific
extensions. Supported emitters: standard, Envoy Gateway, and Kgateway.
(#265, #273, #305, #320)

### Extensive ingress-nginx Annotation Support

Added translation for many new ingress-nginx annotations covering header
manipulation, CORS, redirects, timeouts, path rewrite, backend TLS, buffer
sizing, IP range control, and more.

### E2E Test Framework

Comprehensive end-to-end test suite with real cluster testing across Ingress
NGINX, Kong, and Istio providers. (#294, #330)

## Changes by Kind

### Feature

- Emitters: pluggable emitter framework with support for standard, Envoy Gateway, kGateway, and GCE targets (#265, #273, #305, #320)
- GCE infrastructure provider with support for internal/external load balancers, Cloud Armor, SSL policies, and health checks (#336)
- Read resources from multiple input files via the `--input-file` flag (#258)
- Upgraded Gateway API to v1.5 (#367)
- ingress-nginx: header manipulation support for upstream-vhost, x-forwarded-prefix, connection-proxy-header, and custom-headers (#283)
- ingress-nginx: backend protocol annotation (HTTP, HTTPS, GRPC, GRPCS) (#286)
- ingress-nginx: extended canary support with header-based and cookie-based routing (#287, #374)
- ingress-nginx: path rewrite via rewrite-target annotation (#288)
- ingress-nginx: timeout annotations (proxy-connect-timeout, proxy-send-timeout, proxy-read-timeout) (#289, #353, #376, #377)
- ingress-nginx: SSL redirect annotation (#290)
- ingress-nginx: permanent and temporal redirect annotations with configurable status codes (#299)
- ingress-nginx: route rule name support (#298)
- ingress-nginx: full CORS configuration (allow-origin, methods, headers, credentials, expose-headers, max-age) (#303, #371)
- ingress-nginx: use-regex annotation support (#307, #344)
- ingress-nginx: backend TLS support via BackendTLSPolicy from proxy-ssl-* annotations (#308)
- ingress-nginx: buffer annotations (proxy-body-size, client-body-buffer-size) (#305)
- ingress-nginx: IP range control via whitelist-source-range and denylist-source-range (#345)
- ingress-nginx: parsed annotations tracking with notifications for unsupported/unparsed annotations (#370, #359)
- Log unparsed annotations (#361)

### Bug or Regression

- Fix deduplicate TLS CertificateRefs in gateway listeners (#378)
- Fix applyTCPTimeouts loop using continue instead of return (#377)
- Fix canary annotation value parsed as bool instead of checking presence (#374)
- Fix reading ingresses from cluster (#357)
- Fix setting proper secret group and kind in certificateRefs (#302)
- Fix data race in NotificationAggregator (#292)

### Other (Cleanup or Flake)

- Migrate to golangci-lint v2 (#323)
- Bump Kong chart to v3.0.2 (#349)
- Resolve hosts outside verifiers (#351)

## v0.5.0

No changes since [v0.5.0-rc1](#v050-rc1)

## v0.5.0-rc1

## Changes by Kind

### Feature

- Add parameter to ingress-nginx provider to select ingress class (#231, @adrianmoisey)
- Add support for kyaml printer (#242, @rikatz)
- Add support for named ports (#222, @gavinkflam)
- Added new fields in IR to track the source Ingress of each HTTPRoute BackendRef.
  - Fixed incorrect canary weight assignment when a Service appeared under different paths in both canary and non-canary Ingresses.
  - Added some more validations to ingress-nginx canary annotation parsing. (#251, @Stevenjin8)
- Adds support for the NGINX provider in ingress2gateway. (#224, @sarthyparty)
- The version of the binary can now be printed with the `version` command (e.g. ingress2gateway version) (#216, @spencerhance)
- Upgraded GatewayAPI to v1.4.0 (#248, @Stevenjin8)

### Bug or Regression

- Fix invocations for OIDC-enabled clusters from kubeconfig (#245, @jpiper)
- Print notification table on stderr (#233, @rikatz)

### Other (Cleanup or Flake)

- Do not require namespace when using input-file flag, default to all-namespaces (#241, @rikatz)

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
