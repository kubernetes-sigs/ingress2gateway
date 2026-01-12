# Emitters: Design and Governance

## Motivation

At KubeCon NA 2025 in Atlanta, SIG Network announced the [retirement of Ingress NGINX](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/), giving users only four months to migrate to another solution.
Ingress NGINX has ~100 custom annotations that extend the Kubernetes Ingress resource that don't have a direct mapping to Gateway API.
Thus, for ingress2gateway to be useful at helping users migrate, we need to output implementation-specific resources to maximize coverage of said annotations.

We expect third-party implementations to write and maintain emitters for their specific Gateway API implementations.
This document outlines the design of the emitter system and the policies around third-party code contributions.

## Architecture

At a high level, ingress2gateway will have two main components: providers and emitters.
Providers will read in Ingress-related resources and output an intermediate representation (IR) that is ingress-implementation-neutral.
Specifically, providers will output `EmitterIR` (standard Gateway API resources and some IR that captures any additional information that cannot be expressed in Standard Gateway API).

There will be a common emitter that reads the `EmitterIR` of the provider and translates it to potentially nonstandard Gateway API resources depending on configuration.
This gives us a common component that will implement logic "use any Gateway API feature vs only use stable Gateway API features".
The common emitter will output `EmitterIR`.

Implementation-specific Emitters will read the `EmitterIR` from the common emitter and output Gateway API resources along with implementation-specific resources.
Both providers and emitters MUST log any information that is lost in translation.
Ideally, when there is new IR and an emitter does not implement it, ingress2gateway should automatically emit notifications.

Note that the emitters MAY modify the Gateway API resources as needed.
That said, emitters MUST NOT output non-Gateway API resources unless absolutely necessary.
For example, once Gateway API's CORS support is stable, ingress2gateway should no longer output [Envoy Gateway CORS](https://gateway.envoyproxy.io/docs/tasks/security/cors/).

The architecture looks as follows.
The `provider` and `emitter` are implementation-specific components.
So the `provider` could be an "Ingress NGINX provider" that understands Ingress NGINX annotations,
and the `emitter` could be an "Envoy Gateway emitter" that outputs Envoy Gateway resources.


```
+------------------------------+
|(Ingress + related Resources) |
+-----+------------------------+
      |
      | 1) provider.ToIR(resources)
      v
+-----+--------+
| Emitter IR 1 |
+-----+--------+
      |
      | 2) CommonEmitter.Emit(IR 1)
      v
+-----+--------+
| Emitter IR 2 |
+-----+--------+
      |
      | 3) emitter.Emit(IR 2)
      v
+-----+-----------------------------+
| Gateway API Resources             |
| Implementation-specific Resources |
+-----------------------------------+
```

## Governance and Maintainability

To ensure long-term stability, we list clear expectations for Gateway API implementations that wish to have custom emitters in ingress2gateway.

* All code that serves a single vendor or implementation ("third-party code") MUST have nominated contacts for security that will respond within at most 3 business days, if we find security issues.
* All third-party code MUST live in individual directories/modules, with `CODEOWNERS` indicating who is responsible for reviewing that code.
* Reviews and issues about third-party code MUST be triaged and responded to within 30 days.

Furthermore, the codeowners of third-party emitters MUST commit to the following responsibilities:

* Update the third-party emitters to keep up with upstream Gateway API changes within 60 days of a new Gateway API release.
* Ensure that the output of third-party emitters matches the scope defined below within 60 days of a new Gateway API release.

Not keeping up with these responsibilities will trigger a removal process for the affected third-party emitters which is as follows:

* In the next minor release, the emitters will be marked as deprecated, triggering a warning to users.
* The emitters will be removed in the following minor release or within 30 days, whichever comes later.
* During this time, ingress2gateway maintainers will reach out to the codeowners with two emails spaced one week apart, notifying them of the impending removal as well as a GitHub issue.

A codeowner can stop the removal process if they demonstrate commitment to the above policy as decided by the maintainers.

Existing providers will be exempted from these policies, but may not be fully integrated into the provider/emitter model.

## Scope

We restrict the output of ingress2gateway to keep the project maintainable.
Emitters MUST only output:

* upstream Gateway API resources
* any GEP 713-compatible Policy resource that is in the scope of upstream Gateway API, but not yet part of it
* any extensionRef-compatible resource that is in scope of upstream Gateway API, but not yet part of it (this includes things like HTTPRoute filter extensionRefs)

as determined by the maintainers of ingress2gateway and the Gateway API maintainers together.

For example, we would allow [EnvoyGateway's `HTTPRouteFilter`](https://gateway.envoyproxy.io/docs/api/extension_types/#httproutefilter)
with a [`replaceRegexMatch`](https://gateway.envoyproxy.io/docs/api/extension_types/#replaceregexmatch):

```yaml
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

But, we would not allow anything resembling raw Envoy config, such as [Istio's EnvoyFilter](https://istio.io/latest/docs/reference/config/networking/envoy-filter/),
because such a resource is out of scope of the Gateway API project.
