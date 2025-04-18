# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# We need all the Make variables exported as env vars.
# Note that the ?= operator works regardless.

# Enable Go modules.
export GO111MODULE=on

# I2GWPKG is the package path for i2gw.  This allows us to propogate the git info
# to the binary via LDFLAGS.
I2GWPKG := $(shell go list .)/pkg/i2gw

# Get the version string from git describe.
# --tags: Use annotated tags.
# --always: Fallback to commit hash if no tag is found.
# --dirty: Append -dirty if the working directory has uncommitted changes.
GIT_VERSION_STRING := $(shell git describe --tags --always --dirty 2>/dev/null)

# Construct the LDFLAGS string to inject the version
LDFLAGS := -ldflags="-X '$(I2GWPKG).Version=$(GIT_VERSION_STRING)'"

# Print the help menu.
.PHONY: help
help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: vet fmt verify test build;$(info $(M)...Begin to test, verify and build this project.) @ ## Test, verify and build this project.

# Run go fmt against code
.PHONY: fmt
fmt: ;$(info $(M)...Begin to run go fmt against code.)  @ ## Run go fmt against code.
	gofmt -w ./pkg ./cmd

# Run go vet against code
.PHONY: vet
vet: ;$(info $(M)...Begin to run go vet against code.)  @ ## Run go vet against code.
	go vet ./pkg/... ./cmd/...

# Run go test against code
.PHONY: test
test: vet;$(info $(M)...Begin to run tests.)  @ ## Run tests.
	go test -race -cover ./pkg/... ./cmd/...

# Build the binary
.PHONY: build
build: vet;$(info $(M)...Build the binary.)  @ ## Build the binary.
	go build $(LDFLAGS) -o ingress2gateway .

# Run static analysis.
.PHONY: verify
verify:
	hack/verify-all.sh -v
