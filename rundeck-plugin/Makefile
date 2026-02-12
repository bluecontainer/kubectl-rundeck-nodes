# Makefile for rundeck-k8s-nodes Rundeck plugin
PLUGIN_NAME := rundeck-k8s-nodes
VERSION := 1.0.0
PLUGIN_ZIP := $(PLUGIN_NAME)-$(VERSION).zip
# Top-level dir inside ZIP must match the ZIP filename (without .zip)
PLUGIN_DIR := $(PLUGIN_NAME)-$(VERSION)

# kubectl-rundeck-nodes source directory (repo root)
KUBECTL_PLUGIN_DIR := ..

# Target architecture (default: linux-amd64 for typical Rundeck servers)
GOOS ?= linux
GOARCH ?= amd64

.PHONY: all clean build build-binary

all: build

# Build the bundled kubectl-rundeck-nodes binary
build-binary:
	@echo "Building kubectl-rundeck-nodes for $(GOOS)/$(GOARCH)..."
	cd $(KUBECTL_PLUGIN_DIR) && \
		GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-X main.version=$(VERSION)" \
		-o ./rundeck-plugin/contents/kubectl-rundeck-nodes \
		./cmd/kubectl-rundeck-nodes

build: build-binary $(PLUGIN_ZIP)

$(PLUGIN_ZIP): plugin.yaml contents/nodes.sh contents/kubectl-rundeck-nodes
	@echo "Building Rundeck plugin: $(PLUGIN_ZIP)"
	chmod +x contents/nodes.sh contents/kubectl-rundeck-nodes
	rm -rf $(PLUGIN_DIR)
	mkdir -p $(PLUGIN_DIR)/contents
	cp plugin.yaml $(PLUGIN_DIR)/
	cp contents/nodes.sh contents/kubectl-rundeck-nodes $(PLUGIN_DIR)/contents/
	zip -r $(PLUGIN_ZIP) $(PLUGIN_DIR)/
	rm -rf $(PLUGIN_DIR)

clean:
	rm -f $(PLUGIN_ZIP)
	rm -f contents/kubectl-rundeck-nodes
	rm -rf $(PLUGIN_DIR)

# Build for different architectures
build-linux-amd64:
	$(MAKE) build GOOS=linux GOARCH=amd64
	mv $(PLUGIN_ZIP) $(PLUGIN_NAME)-$(VERSION)-linux-amd64.zip

build-linux-arm64:
	$(MAKE) build GOOS=linux GOARCH=arm64
	mv $(PLUGIN_ZIP) $(PLUGIN_NAME)-$(VERSION)-linux-arm64.zip

build-all: build-linux-amd64 build-linux-arm64

# Install to Rundeck libext directory (requires RUNDECK_HOME)
install: $(PLUGIN_ZIP)
ifndef RUNDECK_HOME
	$(error RUNDECK_HOME is not set)
endif
	cp $(PLUGIN_ZIP) $(RUNDECK_HOME)/libext/
	@echo "Plugin installed to $(RUNDECK_HOME)/libext/"
