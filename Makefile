# Needs to be defined before including Makefile.common to auto-generate targets
DOCKER_ARCHS ?= amd64 armv7 arm64
DOCKER_REPO  ?= webgrip

# Go module lives under src/
GO_MODULE_DIR ?= ./src

include Makefile.common

DOCKER_IMAGE_NAME ?= twitch-exporter

promu-build:
	@echo ">> running promu crossbuild -v"
	cd $(GO_MODULE_DIR) && promu crossbuild -v

# Override Makefile.common targets to execute within the Go module directory.
.PHONY: common-style
common-style:
	@echo ">> checking code style"
	@cd $(GO_MODULE_DIR) && fmtRes=$$($(GOFMT) -d $$(find . -path ./vendor -prune -o -name '*.go' -print)); \
	if [ -n "$${fmtRes}" ]; then \
		echo "gofmt checking failed!"; echo "$${fmtRes}"; echo; \
		echo "Please ensure you are using $$($(GO) version) for formatting code."; \
		exit 1; \
	fi

.PHONY: common-format
common-format:
	@echo ">> formatting code"
	cd $(GO_MODULE_DIR) && $(GO) fmt ./...

.PHONY: common-deps
common-deps:
	@echo ">> getting dependencies"
	cd $(GO_MODULE_DIR) && $(GO) mod download

.PHONY: update-go-deps
update-go-deps:
	@echo ">> updating Go dependencies"
	cd $(GO_MODULE_DIR) && \
	for m in $$($(GO) list -mod=readonly -m -f '{{ if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}' all); do \
		$(GO) get -d $$m; \
	done && \
	$(GO) mod tidy

.PHONY: common-vet
common-vet:
	@echo ">> vetting code"
	cd $(GO_MODULE_DIR) && $(GO) vet $(GOOPTS) ./...

.PHONY: common-test-short
common-test-short: $(GOTEST_DIR)
	@echo ">> running short tests"
	cd $(GO_MODULE_DIR) && $(GOTEST) -short $(GOOPTS) ./...

.PHONY: common-test
common-test: $(GOTEST_DIR)
	@echo ">> running all tests"
	cd $(GO_MODULE_DIR) && $(GOTEST) $(test-flags) $(GOOPTS) ./...

.PHONY: common-unused
common-unused:
	@echo ">> running check for unused/missing packages in go.mod"
	cd $(GO_MODULE_DIR) && $(GO) mod tidy
	@git diff --exit-code -- $(GO_MODULE_DIR)/go.sum $(GO_MODULE_DIR)/go.mod

.PHONY: common-lint
common-lint: $(GOLANGCI_LINT)
ifdef GOLANGCI_LINT
	@echo ">> running golangci-lint"
	cd $(GO_MODULE_DIR) && $(GOLANGCI_LINT) run $(GOLANGCI_LINT_OPTS) ./...
endif

.PHONY: common-lint-fix
common-lint-fix: $(GOLANGCI_LINT)
ifdef GOLANGCI_LINT
	@echo ">> running golangci-lint fix"
	cd $(GO_MODULE_DIR) && $(GOLANGCI_LINT) run --fix $(GOLANGCI_LINT_OPTS) ./...
endif

.PHONY: common-build
common-build: promu
	@echo ">> building binaries"
	cd $(GO_MODULE_DIR) && $(PROMU) build --prefix $(PREFIX) $(PROMU_BINARIES)

.PHONY: common-tarball
common-tarball: promu
	@echo ">> building release tarball"
	cd $(GO_MODULE_DIR) && $(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)
