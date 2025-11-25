# Emitters

## Motivation

At Kubecon 2025, SIG Networking announced the [retirement of Ingress NGINX](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/) giving users only four months to migrate to another solution
There is general consensus that the ingress2gateway migration tool should be the centerpiece of this migration.

For ingress2gateway to be useful, we need to output implementation-specific resources.
We expect third-party implementation to write and maintain this code.
Thus, we need explicit processes and expectations to govern and maintain third-party code.

## Proposal

Currently, ingress2gateway supports specifying an ingress provider (Ingress NGINX, NGINX Ingress, etc) as input.
This allows reading configuration and annotations specific to that Ingress implementation.
We should a configuration vector allows using a specific emitter that outputs implementation-specific resources.

## Architecture

Even though we have a strong focus on Ingress NGINX, we should try to be ingress provider-neutral.
That is, an input provider should translate the input Ingress resources into as much Gateway API as possible and some other form of Ingress-neutral IR.
Then, the emitter will use the Gateway API resources and IR to output Gateway API and implementation specific resources.
Note that the emitters may modify the Gateway API resources as needed.
That said, emitters should not output non-Gateway API resources unless absolutely necessary.
For example, once Gateway API's CORS support is stable, we should not output [Envoy Gateway CORS](https://gateway.envoyproxy.io/docs/tasks/security/cors/)

The API should make it easy for emitters to notify if something is not supported.
Ideally, when new IR is added and an emitter does not implement that, the tool should automatically emit notifications.

## Governance and Maintainability

Although the Ingress NGINX retirement is on the horizon, organizations are slow to migrate infrastructure, even if that means running EOL software.
As such, we should think about the maintainability of ingress2gateway and provide clear expectations for Gateway API implementations that wish to have custom emitters in ingress2gateway

All code that serves a single vendor or implementation ("third-party code") MUST have nominated contacts for security that will respond within at most 3 business days, if we find security issues.

All third-party code MUST live in individual directories/modules, with CODEOWNERS indicating who is responsible for reviewing that code.
Reviews MUST be done in a timely fashion.

Not keeping up with these responsibilities will trigger a removal process for the affected third-party code which is as follows:

* In the next minor release, the code will be marked as deprecated triggering a warning to users.
* The code will be removed in the following minor release or within 30 days, whichever comes later.
* During this time, we will send two emails and create an issue tagging the codeowner

The codeowner can stop the removal process if they demonstrate commitment to the above policy as decided by the maintainers.

## Scope

To keep ingress2gateway from blowing up, we restrict the output ingress2gateway to

* upstream Gateway API resources
* any GEP 713-compatible resource that is in the scope of upstream Gateway API as decided by the maintainers of ingress2gateway.

For example, we would allow [`HTTPRouteFilter`](https://gateway.envoyproxy.io/docs/api/extension_types/#httproutefilter)
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

But, we would not allow anything resembling raw Envoy config, such as [Istio's EnvoyFilter](https://istio.io/latest/docs/reference/config/networking/envoy-filter/).
