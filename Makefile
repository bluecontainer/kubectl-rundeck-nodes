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

.PHONY: all build test clean fmt tidy install cross-compile docker-build docker-buildx docker-buildx-local docker-buildx-setup

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

# Docker image name
DOCKER_IMAGE ?= bluecontainer/kubectl-rundeck-nodes
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

# Run locally
run:
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kubectl-rundeck-nodes
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)
