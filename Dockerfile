# Multi-arch Dockerfile for kubectl-rundeck-nodes
# Discovers Kubernetes workloads and outputs Rundeck resource model JSON
#
# Build:
#   docker build -t bluecontainer/kubectl-rundeck-nodes:latest .
#
# Multi-arch build with buildx:
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t bluecontainer/kubectl-rundeck-nodes:latest --push .

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# Build arguments for multi-arch support
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /workspace

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary for the target platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-X main.version=${VERSION}" \
    -o /kubectl-rundeck-nodes ./cmd/kubectl-rundeck-nodes

# Runtime stage - minimal image
FROM alpine:3.19

# Install ca-certificates for HTTPS connections to K8s API
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -u 1001 rundeck
USER 1001

# Copy the binary from builder
COPY --from=builder /kubectl-rundeck-nodes /usr/local/bin/kubectl-rundeck-nodes

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/kubectl-rundeck-nodes"]
