# Makefile for kubectl-rundeck-nodes
BINARY_NAME := kubectl-rundeck-nodes
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Build output directory
BUILD_DIR := bin

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt

.PHONY: all build test clean fmt lint tidy install cross-compile release release-patch docker-build docker-buildx docker-buildx-local docker-buildx-setup integration-test

all: build

build:
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kubectl-rundeck-nodes

test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

fmt:
	$(GOFMT) ./...

lint:
	@echo "Checking formatting..."
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "Files not formatted:"; echo "$$unformatted"; exit 1; fi
	@echo "Running go vet..."
	$(GOCMD) vet ./...

tidy:
	$(GOMOD) tidy

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install to GOPATH/bin
install:
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) ./cmd/kubectl-rundeck-nodes

# Cross-compile for multiple platforms
cross-compile: clean
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/kubectl-rundeck-nodes
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/kubectl-rundeck-nodes
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/kubectl-rundeck-nodes
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/kubectl-rundeck-nodes
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/kubectl-rundeck-nodes

# Tag and push to trigger the GitHub Actions release workflow.
# Usage: make release RELEASE_VERSION=1.2.3
#        make release-patch  (auto-increment patch: v1.2.3 → v1.2.4)
release:
ifndef RELEASE_VERSION
	$(error Usage: make release RELEASE_VERSION=x.y.z)
endif
	@if git rev-parse "v$(RELEASE_VERSION)" >/dev/null 2>&1; then \
		echo "Error: tag v$(RELEASE_VERSION) already exists"; exit 1; \
	fi
	@echo "Tagging v$(RELEASE_VERSION) and pushing to trigger release workflow..."
	git tag -a "v$(RELEASE_VERSION)" -m "Release v$(RELEASE_VERSION)"
	git push origin "v$(RELEASE_VERSION)"
	@echo "Release workflow triggered: https://github.com/$$(git remote get-url origin | sed 's|.*github.com[:/]||;s|\.git$$||')/actions"

release-patch:
	@LATEST=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
	if [ -z "$$LATEST" ]; then \
		echo "No existing tags found. Use: make release RELEASE_VERSION=0.1.0"; exit 1; \
	fi; \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f2); \
	PATCH=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f3); \
	NEXT="$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
	echo "Latest tag: $$LATEST → next: v$$NEXT"; \
	$(MAKE) release RELEASE_VERSION=$$NEXT

# Docker image name
DOCKER_IMAGE ?= ghcr.io/bluecontainer/kubectl-rundeck-nodes
DOCKER_TAG ?= $(VERSION)
PLATFORMS ?= linux/amd64,linux/arm64

# Build Docker image (single platform, local)
docker-build:
	docker build --build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest .

# Build and push multi-arch Docker image (requires docker buildx)
docker-buildx:
	docker buildx build --platform $(PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		--push .

# Build multi-arch Docker image locally (load into docker, single platform)
docker-buildx-local:
	docker buildx build --load \
		--build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest .

# Create buildx builder if not exists
docker-buildx-setup:
	docker buildx create --name multiarch --use --bootstrap || docker buildx use multiarch

# Run integration tests (requires docker)
integration-test:
	$(MAKE) -C integration-test test

# Run locally
run:
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kubectl-rundeck-nodes
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)
