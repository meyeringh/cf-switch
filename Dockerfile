# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /src

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies with cache mount
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy source code
COPY . .

# Build the binary with cache mounts
ARG TARGETARCH=amd64
ARG TARGETOS=linux
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-w -s" -o cf-switch ./cmd/cf-switch

# Final stage - distroless base image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the binary from builder stage
COPY --from=builder /src/cf-switch /cf-switch

# Use non-root user
USER 65532:65532

# Set metadata
LABEL org.opencontainers.image.title="cf-switch"
LABEL org.opencontainers.image.description="Cloudflare WAF Custom Rule toggle service"
LABEL org.opencontainers.image.source="https://github.com/meyeringh/cf-switch"
LABEL org.opencontainers.image.licenses="GPL-3.0"

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/cf-switch"]
