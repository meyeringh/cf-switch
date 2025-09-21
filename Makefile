# CF-Switch Makefile

.PHONY: build test docker helm-lint helm-package lint fmt vet mod-tidy release help

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

# Create a git tag and update helm chart
release:
	@if [ -z "$(NEW_VERSION)" ]; then echo "Usage: make release NEW_VERSION=1.2.3"; exit 1; fi
	@echo "Creating release $(NEW_VERSION)..."
	@git tag -a $(NEW_VERSION) -m "Release $(NEW_VERSION)"
	@echo "Updating Helm chart version to $(NEW_VERSION)..."
	@sed -i 's/^version: .*/version: $(NEW_VERSION)/' $(HELM_CHART)/Chart.yaml
	@sed -i 's/^appVersion: .*/appVersion: "$(NEW_VERSION)"/' $(HELM_CHART)/Chart.yaml
	@echo "Updated Helm chart to version $(NEW_VERSION)"
	@echo "Committing Helm chart updates..."
	@git add $(HELM_CHART)/Chart.yaml
	@git commit -m "Update Helm chart to version $(NEW_VERSION)" || true
	@echo "Pushing tag $(NEW_VERSION) to origin..."
	@git push origin $(NEW_VERSION)
	@echo "Release $(NEW_VERSION) created and pushed! GitHub Actions should now be running."

# Help
help:
	@echo "Available targets:"
	@echo "  build           Build the binary"
	@echo "  docker          Build docker image locally"
	@echo "  lint            Running Tests and then Linting everything"
	@echo "  dev-build       Build with race detection"
	@echo "  install-tools   Install development tools"
	@echo "  release         Create git tag, update Helm chart, and push to trigger CI/CD (usage: make release NEW_VERSION=1.2.3)"
	@echo "  help            Show this help"
