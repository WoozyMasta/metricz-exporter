SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c

RELEASE_MATRIX ?= darwin/amd64 darwin/arm64 linux/386 linux/amd64 linux/arm linux/arm64 windows/386 windows/amd64

GOTAGS      ?= forceposix
GOFLAGS     ?= -trimpath
LDFLAGS     ?= -s -w
GOWORK      ?= off
CGO_ENABLED ?= 0

BIN_NAME    ?= metricz-exporter
WORK_DIR    ?= ./cmd/metricz-exporter
BIN_DIR     ?= build

NATIVE_GOOS      := $(shell go env GOOS)
NATIVE_GOARCH    := $(shell go env GOARCH)
NATIVE_EXTENSION := $(if $(filter $(NATIVE_GOOS),windows),.exe,)

WINRES_BASE     ?= ./winres/winres.json
WINRES_UPDATED  ?= ./winres/winres.current.json

GO            ?= go
GOLANGCI_LINT ?= golangci-lint
BETTERALIGN   ?= betteralign
CYCLONEDX     ?= cyclonedx-gomod
WINRES        ?= go-winres

# Container settings
GH_USER         ?= woozymasta
DOCKER_USER     ?= woozymasta
REGISTRY_GHCR   := ghcr.io/$(GH_USER)
REGISTRY_DOCKER := docker.io/$(DOCKER_USER)
IMG_BASE_GHCR   := $(REGISTRY_GHCR)/$(BIN_NAME)
IMG_BASE_DOCKER := $(REGISTRY_DOCKER)/$(BIN_NAME)

# Readme
CONFIG_FILE := internal/config/example-config.yaml
README_FILE := README.md

# Versioning details
MODULE    := $(shell $(GO) list -m)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REVISION  := $(shell git rev-list --count HEAD 2>/dev/null || echo 0)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
NAME      := $(shell echo $(BIN_NAME) | sed 's/[-_]/ /g' | awk '{for(i=1;i<=NF;i++)sub(/./,toupper(substr($$i,1,1)),$$i)}1')

# Linker flags to inject variables into internal/vars
LDFLAGS := -s -w \
	-X '$(MODULE)/internal/vars.Name=$(NAME)' \
	-X '$(MODULE)/internal/vars.Version=$(VERSION)' \
	-X '$(MODULE)/internal/vars._revision=$(REVISION)' \
	-X '$(MODULE)/internal/vars.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/vars._buildTime=$(BUILDTIME)' \
	-X '$(MODULE)/internal/vars.URL=https://$(MODULE)'

.DEFAULT_GOAL := build

.PHONY: all build container container-push release deps clean build-dir fmt vet lint align align-fix lint check winres tools geodb update-readme

all: tools check build

build: clean winres build-dir
	@goos="$(GOOS)"; goarch="$(GOARCH)"; \
	GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH) \
	GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" -tags $(GOTAGS) $(EXTRA_BUILD_FLAGS) \
			-o $(BIN_DIR)/$(BIN_NAME)$(NATIVE_EXTENSION) $(WORK_DIR)
	@$(MAKE) _winres_patch GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH) BIN=$(BIN_NAME) OUTEXT="$(NATIVE_EXTENSION)"

container:
	@echo ">> Building container image ($(IMG_BASE_GHCR):$(VERSION))..."
	@docker build -f Dockerfile \
		-t $(IMG_BASE_GHCR):$(VERSION) \
		$(if $(filter-out dev,$(VERSION)),-t $(IMG_BASE_GHCR):latest,) \
		-t $(IMG_BASE_DOCKER):$(VERSION) \
		$(if $(filter-out dev,$(VERSION)),-t $(IMG_BASE_DOCKER):latest,) \
		.

container-push:
	@echo ">> Pushing container image..."
	docker push $(IMG_BASE_GHCR):$(VERSION)
	docker push $(IMG_BASE_DOCKER):$(VERSION)
ifneq ($(VERSION),dev)
	docker push $(IMG_BASE_GHCR):latest
	docker push $(IMG_BASE_DOCKER):latest
endif

release: clean winres build-dir
	@echo ">> Building release binaries..."
	@for target in $(RELEASE_MATRIX); do \
		goos=$${target%%/*}; \
		goarch=$${target##*/}; \
		ext=$$( [ $$goos = "windows" ] && echo ".exe" || echo "" ); \
		out="$(BIN_DIR)/$(BIN_NAME)-$${goos}-$${goarch}$$ext"; \
		echo ">> building $$out"; \
		GOOS=$$goos GOARCH=$$goarch \
		GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
			$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" -tags $(GOTAGS) -o $$out $(WORK_DIR) ; \
		$(MAKE) _winres_patch GOOS=$$goos GOARCH=$$goarch BIN=$(BIN_NAME)-$${goos}-$${goarch} OUTEXT="$$ext"; \
		$(MAKE) _sbom_bin_one GOOS=$$goos GOARCH=$$goarch BIN=$(BIN_NAME)-$${goos}-$${goarch} OUTEXT="$$ext"; \
	done
	@$(MAKE) sbom-app
	@echo ">> Release build complete."

changelog:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } found { print } \
	' CHANGELOG.md

deps:
	@echo ">> Downloading dependencies..."
	@go mod tidy
	@go mod download

clean:
	@echo ">> Cleaning..."
	@rm -rf $(BIN_DIR)

build-dir:
	@echo ">> Make $(BIN_DIR) dir..."
	@mkdir -p $(BIN_DIR)

fmt:
	@echo ">> Running go fmt..."
	@go fmt ./...

vet:
	@echo ">> Running go vet..."
	@go vet ./...

align:
	@echo ">> Checking struct alignment..."
	@betteralign ./...

align-fix:
	@echo ">> Optimizing struct alignment..."
	@betteralign -apply ./...

lint:
	@echo ">> Running golangci-lint..."
	@golangci-lint run

check: fmt vet align lint
	@echo ">> All checks passed."

sbom-app:
	@echo ">> SBOM (app)"
	$(CYCLONEDX) app -json -packages -files -licenses \
		-output "$(BIN_DIR)/$(BIN_NAME).sbom.json" -main $(WORK_DIR)

winres:
	@chmod +x ./winres/winres.sh
	./winres/winres.sh "$(BIN_NAME)" "$(WINRES_BASE)" "$(WINRES_UPDATED)"

tools: tool-golangci-lint tool-betteralign tool-cyclonedx tools-winres
	@echo ">> installing all go tools"

tool-golangci-lint:
	@echo ">> installing golangci-lint"
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

tool-betteralign:
	@echo ">> installing betteralign"
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

tool-cyclonedx:
	@echo ">> installing cyclonedx-gomod"
	$(GO) install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest

tools-winres:
	@echo ">> installing go-winres"
	$(GO) install github.com/tc-hib/go-winres@latest

geodb:
	@echo ">>> downloading GeoLite2-City.mmdb"
	curl -#SfLo GeoLite2-City.mmdb https://git.io/GeoLite2-City.mmdb
	@echo ">>> downloading GeoLite2-Country.mmdb"
	curl -#SfLo GeoLite2-Country.mmdb https://git.io/GeoLite2-Country.mmdb

update-readme:
	@echo "Updating $(README_FILE) with content from $(CONFIG_FILE)..."
	@awk ' \
		/<!-- include:start -->/ { \
			print; \
			print "```yaml"; \
			while ((getline line < "$(CONFIG_FILE)") > 0) print line; \
			close("$(CONFIG_FILE)"); \
			print "```"; \
			skip=1; \
			next \
		} \
		/<!-- include:end -->/ { skip=0 } \
		!skip { print } \
	' $(README_FILE) > $(README_FILE).tmp && mv $(README_FILE).tmp $(README_FILE)
	@echo "Done."

# helpers
_sbom_bin_one:
	@bin="$(BIN_DIR)/$(BIN)$(OUTEXT)"; \
	if [ -f "$$bin" ]; then \
		echo ">> SBOM (bin) $$bin"; \
		$(CYCLONEDX) bin -json -output "$$bin.sbom.json" "$$bin"; \
	fi

_winres_patch:
	@if [ "$(GOOS)" = "windows" ] && [ -f "$(WINRES_UPDATED)" ]; then \
		echo ">> patch winres for $(BIN)$(OUTEXT)"; \
		$(WINRES) patch --no-backup --in "$(WINRES_UPDATED)" \
			"$(BIN_DIR)/$(BIN)$(OUTEXT)"; \
	fi
