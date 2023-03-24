# This is a wrapper to build golang binaries
#
# All make targets related to golang are defined in this file.

VERSION_PACKAGE := github.com/kubernetes-sigs/ingress2gateway/pkg/cmd/version

GO_LDFLAGS += -X $(VERSION_PACKAGE).version=$(shell cat VERSION) \
	-X $(VERSION_PACKAGE).gitCommitID=$(GIT_COMMIT)

GIT_COMMIT:=$(shell git rev-parse HEAD)

GOPATH := $(shell go env GOPATH)
ifeq ($(origin GOBIN), undefined)
	GOBIN := $(GOPATH)/bin
endif

GO_VERSION = $(shell grep -oE "^go [[:digit:]]*\.[[:digit:]]*" go.mod | cut -d' ' -f2)

# Build the target binary in target platform.
# The pattern of build.% is `build.{Platform}.{Command}`.
# If we want to build i2gw in linux amd64 platform, 
# just execute make go.build.linux_amd64.i2gw.
.PHONY: go.build.%
go.build.%:
	@$(LOG_TARGET)
	$(eval COMMAND := $(word 2,$(subst ., ,$*)))
	$(eval PLATFORM := $(word 1,$(subst ., ,$*)))
	$(eval OS := $(word 1,$(subst _, ,$(PLATFORM))))
	$(eval ARCH := $(word 2,$(subst _, ,$(PLATFORM))))
	@$(call log, "Building binary $(COMMAND) with commit $(REV) for $(OS) $(ARCH)")
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(OUTPUT_DIR)/$(OS)/$(ARCH)/$(COMMAND) -ldflags "$(GO_LDFLAGS)" $(ROOT_PACKAGE)/cmd/$(COMMAND)

# Build the i2gw binaries in the hosted platforms.
.PHONY: go.build
go.build: $(addprefix go.build., $(addprefix $(PLATFORM)., $(BINS)))

# Build the i2gw binaries in multi platforms
# It will build the linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 binaries out.
.PHONY: go.build.multiarch
go.build.multiarch: $(foreach p,$(PLATFORMS),$(addprefix go.build., $(addprefix $(p)., $(BINS))))

.PHONY: go.test.unit
go.test.unit: ## Run go unit tests
	go test ./...

.PHONY: go.test.coverage
go.test.coverage:## Run go unit coverage tests in GitHub Actions
	@$(LOG_TARGET)
	go test ./pkg/... -race -v -coverprofile=coverage.xml -covermode=atomic

.PHONY: go.clean
go.clean: ## Clean the building output files
	@$(LOG_TARGET)
	rm -rf $(OUTPUT_DIR)

.PHONY: go.mod.lint
lint: go.mod.lint
go.mod.lint:
	@$(LOG_TARGET)
	@go mod tidy -compat=$(GO_VERSION)
	@if test -n "$$(git status -s -- go.mod go.sum)"; then \
		git diff --exit-code go.mod; \
		git diff --exit-code go.sum; \
		$(call errorlog, "Error: ensure all changes have been committed!"); \
		exit 1; \
	else \
		$(call log, "Go module looks clean!"); \
   	fi

##@ Golang

.PHONY: build
build: ## Build i2gw for host platform. See Option PLATFORM and BINS.
build: go.build

.PHONY: build-multiarch
build-multiarch: ## Build i2gw for multiple platforms. See Option PLATFORMS and IMAGES.
build-multiarch: go.build.multiarch

.PHONY: test
test: ## Run all Go test of code sources.
test: go.test.unit

.PHONY: format
format: ## Update and check dependences with go mod tidy.
format: go.mod.lint

.PHONY: clean
clean: ## Remove all files that are created during builds.
clean: go.clean
