# Apache APISIX Provider

The project supports translating Apache APISIX specific annotations.

## Supported Annotations

- `k8s.apisix.apache.org/http-to-https`: When set to true, this annotation can be used to redirect HTTP requests to HTTPS with a `301` status code and with the same URI as the original request.
