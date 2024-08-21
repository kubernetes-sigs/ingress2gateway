# Adding Providers
This document will guide you through the process of creating a concrete resource converter that implements the `Provider`
interface in the `i2gw` package.

## Overview
Each provider implementation in the `i2gw/providers` package is responsible for converting a provider specific `Ingress`
and related resources (e.g istio VirtualService) into `Gateway API` resources.
A provider must be able to read its custom resources, and convert them.

## Prerequisites
* Familiarity with Go programming language.
* Basic understanding of Kubernetes and its custom resources.
* A setup Go development environment.

## Step-by-Step Implementation
In this section, we will walk through a demo of how to add support for the `example-gateway` provider.

1. Add a new package under the `providers` package. Say we want to add a new gateway provider example.
```
.
├── ingress2gateway.go
├── ingress2gateway_test.go
├── provider.go
└── providers
    ├── common
    ├── examplegateway
    └── ingressnginx
```
2. Create a struct named `resourceReader` which implements the `CustomResourceReader` interface in a file named
`resource_converter.go`.
```go
package examplegateway

import (
	"context"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

// converter implements the i2gw.CustomResourceReader interface.
type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	return &resourceReader{
		conf: conf,
	}
}

func (r *resourceReader) ReadResourcesFromCluster(ctx context.Context) error {
	// read example-gateway related resources from the cluster.
	return nil
}

func (r *resourceReader) ReadResourcesFromFiles(ctx context.Context, filename string) error {
	// read example-gateway related resources from the file.
	return nil
}
```

These methods are used by providers to read and store additional resources they may need during conversion.

3. Create a struct named `converter` which implements the `ResourceConverter` interface in a file named `converter.go`.
The implemented `ToGatewayAPI` function should simply call every registered `featureParser` function, one by one.
Take a look at `ingressnginx/converter.go` for an example.
The `ImplementationSpecificOptions` struct contains the handlers to customize native ingress implementation-specific fields.
Take a look at `kong/converter.go` for an example.

```go
package examplegateway

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	conf *i2gw.ProviderConf

	featureParsers []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

// newConverter returns an ingress-nginx converter instance.
func newConverter(conf *i2gw.ProviderConf) *converter {
	return &converter{
		conf: conf,
		featureParsers: []i2gw.FeatureParser{
			// The list of feature parsers comes here.
		},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			// The list of the implementationSpecific ingress fields options comes here.
		},
	}
}
```
4. Create a new struct named after the provider you are implementing. This struct should embed the previous 2 structs
you created.
```go
package examplegateway

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

// Provider implements the i2gw.Provider interface.
type Provider struct {
	conf *i2gw.ProviderConf

	*resourceReader
	*converter
}

// NewProvider constructs and returns the example-gateway implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		conf:           conf,
		resourceReader: newResourceReader(conf),
		converter:      newConverter(conf),
	}
}
```
5. Add the new provider to `i2gw.ProviderConstructorByName`.
```go
package examplegateway

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

// The Name of the provider.
const Name = "example-gateway-provider"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}
```
6. [optional] In order to use notification mechanism, create a `notify` function in a file named `notification.go`. This method is used to reduce the function signature for creating notifications during the conversion process.
```go
package examplegateway

import "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/notifications"

func notify(mType notifications.MessageType, message string, callingObjects ...client.Object){
	newNotification := notifications.NewNotification(mType, message, callingObjects...)
	notifications.NotificationAggr.DispatchNotification(newNotification, string(ProviderName))
}
```
7. Import the new package at `cmd/print`.
```go
package cmd

import (
	// Call init function for the providers
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/examplegateway"
)
```

## Creating a feature parser
In case you want to add support for the conversion of a specific feature within a provider (see for example the canary
feature of ingress-nginx) you'll want to implement a `FeatureParser` function.

Different `FeatureParsers` within the same provider will run in undetermined order. This means that when building a
`Gateway API` resource manifest, you cannot assume anything about previously initialized fields.
The function must modify / create only the required fields of the resource manifest and nothing else.

For example, lets say we are implementing the canary feature of some provider. When building the `HTTPRoute`, we cannot
assume that the `BackendRefs` is already initialized with every `BackendRef` required. The canary `FeatureParser`
function must add every missing `BackendRef` and update existing ones.

### Testing the feature parser
There are 2 main things that needs to be tested when creating a feature parser:
1. The conversion logic is actually correct.
2. The new function doesn't override other functions modifications.
For example, if one implemented the mirror backend feature and it deletes canary weight from `BackendRefs`, we have a
problem.

## Provider-specific flags
To define provider-specific flags the user can supply in the `print` command, call the
`i2gw.RegisterProviderSpecificFlag(ProviderName, i2gw.ProviderSpecificFlag)` function in the init function of the
provider. E.g.:
```go
const Name = "example-gateway-provider"

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider

	i2gw.RegisterProviderSpecificFlag(ProviderName, i2gw.ProviderSpecificFlag{
		Name:         "infrastructure-labels",
		Description:  "Comma-separated list of Gateway infrastructure key=value labels",
    DefaultValue: "",
	})
}
```
Users can provide a value to the flag as follows:
```sh
./ingress2gateway print --providers=example-gateway-provider --example-gateway-provider-infrastructure-labels="app=my-app"
```
The values of all provider-specific flags supplied by the user can be retrieved from the provider `conf`:
```go
if ps := conf.ProviderSpecificFlags[ProviderName]; ps != nil {
  labels := ps["infrastructure-labels"]
}
```
