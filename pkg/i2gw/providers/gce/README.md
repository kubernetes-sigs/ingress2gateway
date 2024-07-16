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
- [Basic Internal Ingress](https://github.com/GoogleCloudPlatform/gke-networking-recipes/tree/main/ingress/single-cluster/ingress-internal-basic)
- [Basic external Ingress](https://github.com/GoogleCloudPlatform/gke-networking-recipes/tree/main/ingress/single-cluster/ingress-external-basic)
- [Ingress with custom default backend](https://github.com/GoogleCloudPlatform/gke-networking-recipes/tree/main/ingress/single-cluster/ingress-custom-default-backend)

To be supported:
 - [Ingress with custom HTTP health check](https://github.com/GoogleCloudPlatform/gke-networking-recipes/tree/main/ingress/single-cluster/ingress-custom-http-health-check)
 - [IAP enabled ingress](https://github.com/GoogleCloudPlatform/gke-networking-recipes/tree/main/ingress/single-cluster/ingress-iap)
 - [Google Cloud Armor enabled ingress](https://github.com/GoogleCloudPlatform/gke-networking-recipes/blob/main/ingress/single-cluster/ingress-cloudarmor/README.md)
 - [Ingress with HTTPS redirect](https://github.com/GoogleCloudPlatform/gke-networking-recipes/tree/main/ingress/single-cluster/ingress-https)

## Summary of GKE Ingress annotation
External Ingress:
https://cloud.google.com/kubernetes-engine/docs/how-to/load-balance-ingress#summary_of_external_ingress_annotations

Internal Ingress:
https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balance-ingress#summary_of_internal_ingress_annotations