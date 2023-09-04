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
1. Add a new package under the `providers` package.
2. Create a struct named `resourceReader` which implements the `CustomResourceReader` interface in a file named
`resource_converter.go`.
3. Create a struct named `converter` which implements the `ResourceConverter` interface in a file named `converter.go`.
The implemented `ToGatewayAPI` function should simply call every registered `featureParser` function, one by one.
Take a look at `ingressnginx/converter.go` for example.
4. Create a new struct named after the provider you are implementing. This struct should embed the previous 2 structs 
you created.
5. Add the new provider to `i2gw.ProviderConstructorByName`.
6. Import the new package at `cmd/print`.

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