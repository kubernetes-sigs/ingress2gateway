# Adding Providers
This document will guide you through the process of creating a concrete resource converter that implements the `Provider` interface in the `i2gw` package.

## Overview
Each provider implementation in the `i2gw/providers` package is responsible for converting a provider specific `Ingress` related resources into `Gateway API` resources.
A provider must be able to read its custom resources, and convert them.

## Prerequisites
* Familiarity with Go programming language.
* Basic understanding of Kubernetes and its custom resources.
* A setup Go development environment.

## Step-by-Step Implementation
1. Add a new package under `providers` package.
2. Create a struct named `resourceReader` which implements the `CustomResourceReader` interface in a file named `resource_converter.go`.
3. Create a struct named `converter` which implements the `ResourceConverter` interface in a file named `converter.go`.
The implemented `ToGatewayAPI` function should simply call every registered `featureParser` function, one by one.
Take a look at `ingressnginx/converter.go` for example.
4. Create a new struct named after the provider you are implementing. This struct should
embed the previous 2 structs you created.
5. Add the new provider to `i2gw.ProviderConstructorByName`.
6. Import the new package at `cmd/print`.

## Creating a feature parser