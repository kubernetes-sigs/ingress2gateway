# Emitters: Design and Policy

## Motivation

At Kubecon 2025, SIG Networking announced the [retirement of Ingress NGINX](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/) giving users only four months to migrate to another solution.
Ingress NGINX has ~100 custom annotations that extend the Kubernetes Ingress resource that don't have a direct mapping to Gateway API.
Thus, for ingress2gateway to be useful at helping users migrate, we need to output implementation-specific resources to maximize coverage of said annotations.

We expect third-party implementations to write and maintain emitters for their specific Gateway API implementations.
This document outlines the design of the emitter system and the policies around third-party code contributions.

## Architecture

At a high level, ingress2gateway will have two main components: Input providers and 0utput emitters.
Providers will read in Ingress-related resources and output an intermediate representation (IR) that is ingress-implementation-neutral.
Specifically, providers will output Gateway API resources and some IR that captures any additional information that cannot be expressed in Gateway API.
Emitters will read the IR and output Gateway API resources and implementation-specific resources.
Both emitters and providers MUST log any information that is lost in translation.
Ideally, when there is new IR and an emitter does not implement it, the tool should automatically emit notifications.

Note that the emitters MAY modify the Gateway API resources as needed.
That said, emitters MUST NOT output non-Gateway API resources unless absolutely necessary.
For example, once Gateway API's CORS support is stable, ingress2gateway should no longer output [Envoy Gateway CORS](https://gateway.envoyproxy.io/docs/tasks/security/cors/)

## Governance and Maintainability

To ensure long-term stability, we list clear expectations for Gateway API implementations that wish to have custom emitters in ingress2gateway.

* All code that serves a single vendor or implementation ("third-party code") MUST have nominated contacts for security that will respond within at most 3 business days, if we find security issues.
* All third-party code MUST live in individual directories/modules, with `CODEOWNERS` indicating who is responsible for reviewing that code.
* Reviews and issues about third-party code MUST be triaged and responded to within 30 days.

Not keeping up with these responsibilities will trigger a removal process for the affected third-party code which is as follows:

* In the next minor release, the code will be marked as deprecated triggering a warning to users.
* The code will be removed in the following minor release or within 30 days, whichever comes later.
* During this time, ingress2gateway maintainers will reach out to the codeowners with two emails spaced one week apart notifying them of the impending removal as well as a GitHub issue.

The codeowner can stop the removal process if they demonstrate commitment to the above policy as decided by the maintainers.

Existing providers will be grandfathered in, but may not be fully integrated into the provider/emitter model.

## Scope

We restrict the output ingress2gateway to keep the project maintainable.
Emitter MUST only output:

* upstream Gateway API resources
* any GEP 713-compatible resource that is in the scope of upstream Gateway API, but not yet part of it.

For example, we would allow [EnvoyGateway's `HTTPRouteFilter`](https://gateway.envoyproxy.io/docs/api/extension_types/#httproutefilter)
with a [`replaceRegexMatch`](https://gateway.envoyproxy.io/docs/api/extension_types/#replaceregexmatch):

```
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: HTTPRouteFilter
metadata:
  name: regex-path-rewrite
spec:
  urlRewrite:
    path:
      type: ReplaceRegexMatch
      replaceRegexMatch:
        pattern: '^/service/([^/]+)(/.*)$'
        substitution: '\2/instance/\1'
```

But, we would not allow anything resembling raw Envoy config, such as [Istio's EnvoyFilter](https://istio.io/latest/docs/reference/config/networking/envoy-filter/), because such a resource conflicts with Gateway API's goal of being implementation-neutral.
