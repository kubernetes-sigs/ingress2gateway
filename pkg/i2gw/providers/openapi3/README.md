# OpenAPI Provider

The provider translates OpenAPI Specification (OAS) 3.x documents to Kubernetes Gateway API resources – Gateway, HTTPRoutes and ReferenceGrants.

## Usage

```sh
./ingress2gateway print --providers=openapi3 --input-file=FILEPATH
```

Where `FILEPATH` is the path to a file containing a valid OpenAPI Specification in YAML or JSON format.

**Gateway class name**

To specify the name of the gateway class for the Gateway resources, use `--openapi3-gateway-class-name=NAME`.

**Gateways with TLS configuration**

If one or more servers specified in the OAS start with `https`, TLS configuration will be added to the corresponding gateway listener.
To specify the reference to the gateway TLS secret, use `--openapi3-gateway-tls-secret=SECRET-NAME` or `--openapi3-gateway-tls-secret=SECRET-NAMESPACE/SECRET-NAME`.

**Backend references**

All routes generated will point to a single backend service.
To specify the backend reference, use `--openapi3-backend=SERVICE-NAME` or `--openapi3-backend=SERVICE-NAMESPACE/SERVICE-NAME`.

Specifying the port number to the backend service is currently not supported.

**Resource names**

Gateway and HTTPRoute names are prefixed with the [title](https://swagger.io/specification/v3/) of the OAS converted to Kubernetes object name format.

In case of multiple resources of a kind, the names are suffixed with the corresponding sequential number from 1.

In all cases, ensure the title of the OAS is not long enough that would cause invalid [Kubernetes object names](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/).

## Examples

The examples below are based on the [Swagger Petstore Sample API](https://petstore3.swagger.io).

```sh
./ingress2gateway print --providers=openapi3 \
                        --openapi3-gateway-class-name=istio \
                        --openapi3-backend=my-app \
                        --input-file=petstore3-openapi.json
```

<details>
  <summary>Expected output</summary>

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  creationTimestamp: null
  name: swagger-petstore-openapi-3-0-gateway
  namespace: default
spec:
  gatewayClassName: istio
  listeners:
  - hostname: '*'
    name: http
    port: 80
    protocol: HTTP
status: {}
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  creationTimestamp: null
  name: swagger-petstore-openapi-3-0-route
  namespace: default
spec:
  parentRefs:
  - name: swagger-petstore-openapi-3-0-gateway
  rules:
  - backendRefs:
    - name: my-app
    matches:
    - method: POST
      path:
        type: Exact
        value: /api/v3/pet
    - method: PUT
      path:
        type: Exact
        value: /api/v3/pet
    - method: GET
      path:
        type: Exact
        value: /api/v3/pet/findByStatus
    - method: GET
      path:
        type: Exact
        value: /api/v3/pet/findByTags
    - method: DELETE
      path:
        type: Exact
        value: /api/v3/pet/{petId}
    - method: GET
      path:
        type: Exact
        value: /api/v3/pet/{petId}
    - method: POST
      path:
        type: Exact
        value: /api/v3/pet/{petId}
    - method: POST
      path:
        type: Exact
        value: /api/v3/pet/{petId}/uploadImage
  - backendRefs:
    - name: my-app
    matches:
    - method: GET
      path:
        type: Exact
        value: /api/v3/store/inventory
    - method: POST
      path:
        type: Exact
        value: /api/v3/store/order
    - method: DELETE
      path:
        type: Exact
        value: /api/v3/store/order/{orderId}
    - method: GET
      path:
        type: Exact
        value: /api/v3/store/order/{orderId}
    - method: POST
      path:
        type: Exact
        value: /api/v3/user
    - method: POST
      path:
        type: Exact
        value: /api/v3/user/createWithList
    - method: GET
      path:
        type: Exact
        value: /api/v3/user/login
    - method: GET
      path:
        type: Exact
        value: /api/v3/user/logout
  - backendRefs:
    - name: my-app
    matches:
    - method: DELETE
      path:
        type: Exact
        value: /api/v3/user/{username}
    - method: GET
      path:
        type: Exact
        value: /api/v3/user/{username}
    - method: PUT
      path:
        type: Exact
        value: /api/v3/user/{username}
status:
  parents: null
```

</details>

ReferenceGrants are only generated if a namespace is specified and/or the references to gateway TLS secrets or backends do not match the target namespace, which can occasionally unspecified. E.g.:

```sh
./ingress2gateway print --providers=openapi3 \
                        --namespace=networking \
                        --openapi3-gateway-class-name=istio \
                        --openapi3-backend=apps/my-app \
                        --input-file=petstore3-openapi.json
```

<details>
  <summary>Expected output</summary>

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  creationTimestamp: null
  name: swagger-petstore-openapi-3-0-gateway
  namespace: networking
spec:
  gatewayClassName: istio
  listeners:
  - hostname: '*'
    name: http
    port: 80
    protocol: HTTP
status: {}
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  creationTimestamp: null
  name: swagger-petstore-openapi-3-0-route
  namespace: networking
spec:
  parentRefs:
  - name: swagger-petstore-openapi-3-0-gateway
  rules:
  - backendRefs:
    - name: my-app
      namespace: apps
    matches:
    - method: POST
      path:
        type: Exact
        value: /api/v3/pet
    - method: PUT
      path:
        type: Exact
        value: /api/v3/pet
    - method: GET
      path:
        type: Exact
        value: /api/v3/pet/findByStatus
    - method: GET
      path:
        type: Exact
        value: /api/v3/pet/findByTags
    - method: DELETE
      path:
        type: Exact
        value: /api/v3/pet/{petId}
    - method: GET
      path:
        type: Exact
        value: /api/v3/pet/{petId}
    - method: POST
      path:
        type: Exact
        value: /api/v3/pet/{petId}
    - method: POST
      path:
        type: Exact
        value: /api/v3/pet/{petId}/uploadImage
  - backendRefs:
    - name: my-app
      namespace: apps
    matches:
    - method: GET
      path:
        type: Exact
        value: /api/v3/store/inventory
    - method: POST
      path:
        type: Exact
        value: /api/v3/store/order
    - method: DELETE
      path:
        type: Exact
        value: /api/v3/store/order/{orderId}
    - method: GET
      path:
        type: Exact
        value: /api/v3/store/order/{orderId}
    - method: POST
      path:
        type: Exact
        value: /api/v3/user
    - method: POST
      path:
        type: Exact
        value: /api/v3/user/createWithList
    - method: GET
      path:
        type: Exact
        value: /api/v3/user/login
    - method: GET
      path:
        type: Exact
        value: /api/v3/user/logout
  - backendRefs:
    - name: my-app
      namespace: apps
    matches:
    - method: DELETE
      path:
        type: Exact
        value: /api/v3/user/{username}
    - method: GET
      path:
        type: Exact
        value: /api/v3/user/{username}
    - method: PUT
      path:
        type: Exact
        value: /api/v3/user/{username}
status:
  parents: null
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  creationTimestamp: null
  name: from-networking-to-service-my-app
  namespace: apps
spec:
  from:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    namespace: networking
  to:
  - group: ""
    kind: Service
    name: my-app
```
</details>

## Limitations

* Only offline translation supported – i.e. `--input-file` is required
* An input file can only declare one OpenAPI Specification
* All API operation [paths](https://swagger.io/specification/v3/#paths-object) are treated as `Exact` type – i.e. no support for [path templating](https://swagger.io/specification/v3/#path-templating), therefore no `PathPrefix`, nor `RegularExpression` path types output
* Limited support for [parameters](https://swagger.io/specification/v3/#parameter-object) – only required `header` and `query` parameters supported
* Limited support to [server variables](https://swagger.io/specification/v3/#server-variable-object) – only limited sets (`enum`) supported
* No support to [references](https://swagger.io/specification/v3/#reference-object) (`$ref`)
* No support to [external documents](https://swagger.io/specification/v3/#external-documentation-object)
* OpenAPI Specification with a large number of server combinations may generate Gateway resources with more listeners than allowed

Additionally, no support to any OpenAPI feature with no direct equivalent to core Gateway API fields, such as [request bodies](https://swagger.io/specification/v3/#request-body-object), [examples](https://swagger.io/specification/v3/#example-object), [security schemes](https://swagger.io/specification/v3/#security-scheme-object), [callbacks](https://swagger.io/specification/v3/#callback-object), [extensions](https://swagger.io/specification/v3/#specification-extensions), etc.
