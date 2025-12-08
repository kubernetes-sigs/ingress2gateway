# GCE Provider

The project supports translating ingress-gce specific annotations.

Currently supported annotations:
`kubernetes.io/ingress.class`: Though it is a deprecated annotation for most providers, GCE still uses this annotation to specify the specific type of load balancers created by GKE Ingress.

## Implementation-specific features

The following implementation-specific features are supported:

- Ingress path with type `ImplementationSpecific` will:
  - Translate to equivalent Gateway Prefix path but dropping `/*`, if `/*` exists
  - Translate to equivalent Exact path otherwise.

Examples:
| Ingress `ImplementationSpecific` Path | map to Gateway Path                    |
| ------------------------------------- | -------------------------------------- |
| /*                                    | / Prefix                               |
| /v1                                   | /v1 Exact                              |
| /v1/                                  | /v1/ Exact                             |
| /v1/*                                 | /v1 Prefix                             |

Note: For Ingress `ImplementationSpecific` path with `/v1/*`, it will map to
`/v1/` or `/v1/v2` but not `/v1`, but the translator will convert it to the
most similar Gateway path `/v1` Prefix, which means while `/v1` path is not
considered as a matching path for Ingress, it will be a matching path for
Gateway.
If you want to avoid such behavior, please consider switching to `Prefix`, 
`Exact`, or an `ImplementationSpecific` path without `*` before converting
an Ingress to a Gateway.

## Feature list
Currently supported:
 - [Internal Ingress](https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balance-ingress)
 - [External Ingress](https://cloud.google.com/kubernetes-engine/docs/how-to/load-balance-ingress)
 - [Custom default backend](https://cloud.google.com/kubernetes-engine/docs/concepts/ingress#default_backend)
 - [Google Cloud Armor Ingress security policy](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#cloud_armor)
 - [SSL Policy](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#ssl) 
 - [Custom health check configuration](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#direct_health)

To be supported:
 - [HTTP-to-HTTPS redirect](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#https_redirect)
 - [Backend Service Timeout](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#timeout)
 - [Cloud CDN](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#cloud_cdn)
 - [Connection Drain Timeout](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#draining_timeout)
 - [HTTP Access Logging](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#http_logging)
 - [Identity-Aware Proxy](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#iap)
 - [Session affinity](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#session_affinity)
 - [User-defined request headers](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#request_headers)
 - [Custom Response header](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#response_headers) 

## Summary of GKE Ingress annotation
External Ingress:
https://cloud.google.com/kubernetes-engine/docs/how-to/load-balance-ingress#summary_of_external_ingress_annotations

Internal Ingress:
https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balance-ingress#summary_of_internal_ingress_annotations