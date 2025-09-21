# CF-Switch Makefile

.PHONY: build test docker helm-lint helm-package lint fmt vet mod-tidy

# Variables
VERSION = $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0-dev")
IMAGE_REPO ?= ghcr.io/meyeringh/cf-switch
IMAGE_TAG ?= $(VERSION)
IMAGE_FULL = $(IMAGE_REPO):$(IMAGE_TAG)
BINARY_NAME = cf-switch
BUILD_DIR = ./bin
HELM_CHART = ./deploy/helm/cf-switch

# Go commands
GO_BUILD = go build
GO_TEST = go test
GO_FMT = go fmt
GO_VET = go vet

# Build
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags="-w -s -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cf-switch

# Docker
docker:
	@echo "Building Docker image $(IMAGE_FULL)..."
	docker build -t $(IMAGE_FULL) .

# Linting and Testing
lint:
	@echo "Running Tests and then Linting everything..."
	$(GO_TEST) -v -race -coverprofile=coverage.out ./...
	$(GO_FMT) ./...
	$(GO_VET) ./...
	$$(go env GOPATH)/bin/golangci-lint run
	helm lint $(HELM_CHART)

# Development build (with race detection)
dev-build:
	@echo "Building for development $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) -race -ldflags="-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cf-switch

# Run locally (for development)
dev-run: dev-build
	@echo "Running cf-switch locally..."
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a && \
		RUNNING_LOCALLY=true ./$(BUILD_DIR)/$(BINARY_NAME); \
	else \
		RUNNING_LOCALLY=true ./$(BUILD_DIR)/$(BINARY_NAME); \
	fi

# Install development tools
install-tools:
	@echo "Installing development tools..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.4.0
# Generate API documentation
generate-docs:
	@echo "API documentation available at api/openapi.yaml"
	@echo "View with: https://editor.swagger.io/"

# Create a git tag and update helm chart
release:
	@if [ -z "$(NEW_VERSION)" ]; then echo "Usage: make release NEW_VERSION=1.2.3"; exit 1; fi
	@echo "Creating release $(NEW_VERSION)..."
	@git tag -a $(NEW_VERSION) -m "Release $(NEW_VERSION)"
	@echo "Updating Helm chart version to $(VERSION)..."
	@sed -i 's/^version: .*/version: $(VERSION:v%=%)/' $(HELM_CHART)/Chart.yaml
	@sed -i 's/^appVersion: .*/appVersion: $(VERSION)/' $(HELM_CHART)/Chart.yaml
	@echo "Updated Helm chart to version $(VERSION)"
	@echo "Release $(NEW_VERSION) created!"
	@echo "Push with: git push origin $(NEW_VERSION)"

# Help
help:
	@echo "Available targets:"
	@echo "  build           Build the binary"
	@echo "  docker          Build docker image locally"
	@echo "  lint            Running Tests and then Linting everything"
	@echo "  dev-build       Build with race detection"
	@echo "  install-tools   Install development tools"
	@echo "  generate-docs   Show API documentation info"
	@echo "  release         Create git tag and update Helm chart (usage: make release NEW_VERSION=v1.2.3)"
	@echo "  help            Show this help"
