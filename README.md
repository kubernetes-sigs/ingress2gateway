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

## Get Involved

This project will be discussed in the same Slack channel and community meetings
as the rest of the Gateway API subproject. For more information, refer to the
[Gateway API Community](https://gateway-api.sigs.k8s.io/contributing/) page.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of
Conduct](code-of-conduct.md).

[owners]: https://git.k8s.io/community/contributors/guide/owners.md
[Creative Commons 4.0]: https://git.k8s.io/website/LICENSE
