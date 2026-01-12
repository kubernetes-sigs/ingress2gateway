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

REPO_ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

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

# Directory for local binaries (used in CI where we can't install to /usr/local/bin).
LOCAL_BIN := $(REPO_ROOT)/bin

KIND_VERSION ?= v0.25.0

# Default arguments for `go test` in e2e tests.
I2GW_GO_TEST_ARGS ?= -race -v -count=1 -timeout=30m

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

# Detect OS and architecture for kind installation
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
	ARCH := amd64
endif
ifeq ($(ARCH),aarch64)
	ARCH := arm64
endif

KIND_BINARY_URL := https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$(OS)-$(ARCH)
KIND := $(shell command -v kind 2>/dev/null || echo "$(LOCAL_BIN)/kind")

.PHONY: ensure-kind
ensure-kind:
	@if command -v kind >/dev/null 2>&1; then \
		echo "Found kind binary: $$(kind version)"; \
	elif [ -x "$(LOCAL_BIN)/kind" ]; then \
		echo "Found kind binary in $(LOCAL_BIN): $$($(LOCAL_BIN)/kind version)"; \
	else \
		echo "kind binary not found. Installing kind $(KIND_VERSION) for $(OS)/$(ARCH)..."; \
		mkdir -p $(LOCAL_BIN); \
		curl -Lo $(LOCAL_BIN)/kind $(KIND_BINARY_URL); \
		chmod +x $(LOCAL_BIN)/kind; \
		echo "kind installed successfully to $(LOCAL_BIN)/kind"; \
	fi

.PHONY: kind
kind: ensure-kind
	@if ! $(KIND) get clusters | grep -q i2gw-e2e; then \
		$(KIND) create cluster -n i2gw-e2e --kubeconfig $(REPO_ROOT)/kind-kubeconfig; \
	else \
		echo "Cluster i2gw-e2e already exists. Reusing it."; \
		$(KIND) get kubeconfig --name i2gw-e2e > $(REPO_ROOT)/kind-kubeconfig; \
	fi

# Set I2GW_KUBECONFIG to a path to a kubeconfig file to run the tests against an existing cluster.
# Running without setting this variable creates a local kind cluster and uses it for running the
# tests.
# See README.md for more info.
.PHONY: e2e
e2e: ## Run end-to-end tests.
	@if [ ! -z "$${KUBECONFIG}" ]; then \
		echo "ERROR: KUBECONFIG is set in current shell. Refusing to run to avoid touching an"; \
		echo "unrelated cluster."; \
		echo "Unset KUBECONFIG and run 'I2GW_KUBECONFIG=/path/to/kubeconfig make e2e' to run"; \
		echo "the tests against an existing cluster, or run 'make e2e' with no vars to use an"; \
		echo "auto-created kind cluster."; \
		exit 1; \
	fi
	@cleanup_kind=false; \
	kubeconfig="$${I2GW_KUBECONFIG}"; \
	if [ -z "$${I2GW_KUBECONFIG}" ]; then \
		$(MAKE) kind || exit 1; \
		kubeconfig="$(REPO_ROOT)/kind-kubeconfig"; \
		cleanup_kind=true; \
	fi; \
	set -x; \
	KUBECONFIG=$${kubeconfig} go test $(I2GW_GO_TEST_ARGS) $(REPO_ROOT)/e2e; \
	test_exit_code=$$?; \
	set +x; \
	if [ "$${cleanup_kind}" = "true" ] && [ "$${SKIP_CLEANUP}" != "1" ]; then \
		$(MAKE) clean-kind; \
	fi; \
	exit $$test_exit_code

.PHONY: clean-kind
clean-kind:
	$(KIND) delete cluster -n i2gw-e2e
	rm -f $(REPO_ROOT)/kind-kubeconfig
