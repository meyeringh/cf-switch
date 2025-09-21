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
GO_CLEAN = go clean
GO_FMT = go fmt
GO_VET = go vet
GO_MOD = go mod

# Build
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags="-w -s -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cf-switch

# Test
test:
	@echo "Running tests..."
	$(GO_TEST) -v -race -coverprofile=coverage.out ./...

# Docker
docker:
	@echo "Building Docker image $(IMAGE_FULL)..."
	docker build -t $(IMAGE_FULL) .

# Helm lint
helm-lint:
	@echo "Linting Helm chart..."
	helm lint $(HELM_CHART)

# Helm package
helm-package:
	@echo "Packaging Helm chart..."
	helm package $(HELM_CHART) -d ./dist

# Helm template (for testing)
helm-template:
	@echo "Templating Helm chart..."
	helm template cf-switch $(HELM_CHART) \
		--set image.repository=$(IMAGE_REPO) \
		--set image.tag=$(IMAGE_TAG) \
		--set env.DEST_HOSTNAMES.value="paperless.meyeringh.org,photos.example.com" \
		--set env.CLOUDFLARE_ZONE_ID.value="your-zone-id-here"

# Lint Go code
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

# Format Go code
fmt:
	@echo "Formatting Go code..."
	$(GO_FMT) ./...

# Vet Go code
vet:
	@echo "Vetting Go code..."
	$(GO_VET) ./...

# Tidy modules
mod-tidy:
	@echo "Tidying modules..."
	$(GO_MOD) tidy

# Download dependencies
mod-download:
	@echo "Downloading dependencies..."
	$(GO_MOD) download

# Check if dependencies are up to date
mod-verify:
	@echo "Verifying module dependencies..."
	$(GO_MOD) verify

# Run all checks
check: fmt vet lint test

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
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

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
	@echo "  test            Run tests"
	@echo "  test-coverage   Run tests with coverage report"
	@echo "  clean           Clean build artifacts"
	@echo "  docker-build    Build Docker image"
	@echo "  docker-push     Push Docker image"
	@echo "  docker-buildx   Build and push multi-arch image"
	@echo "  helm-lint       Lint Helm chart"
	@echo "  helm-package    Package Helm chart"
	@echo "  helm-template   Generate Helm templates"
	@echo "  lint            Run golangci-lint"
	@echo "  fmt             Format Go code"
	@echo "  vet             Vet Go code"
	@echo "  mod-tidy        Tidy Go modules"
	@echo "  mod-download    Download dependencies"
	@echo "  mod-verify      Verify dependencies"
	@echo "  check           Run all checks (fmt, vet, lint, test)"
	@echo "  dev-build       Build with race detection"
	@echo "  dev-run         Run locally for development"
	@echo "  install-tools   Install development tools"
	@echo "  generate-docs   Show API documentation info"
	@echo "  release         Create git tag and update Helm chart (usage: make release NEW_VERSION=v1.2.3)"
	@echo "  help            Show this help"
