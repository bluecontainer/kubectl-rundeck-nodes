# kubectl-rundeck-nodes

[![CI](https://github.com/bluecontainer/kubectl-rundeck-nodes/actions/workflows/ci.yaml/badge.svg)](https://github.com/bluecontainer/kubectl-rundeck-nodes/actions/workflows/ci.yaml)
[![Release](https://github.com/bluecontainer/kubectl-rundeck-nodes/actions/workflows/release.yaml/badge.svg)](https://github.com/bluecontainer/kubectl-rundeck-nodes/actions/workflows/release.yaml)

A kubectl plugin and Go library that discovers Kubernetes workloads (Helm releases, StatefulSets, Deployments, and Pods) and outputs them as Rundeck resource model JSON. This enables Rundeck to dynamically discover and manage Kubernetes workloads without manual node configuration.

The project includes a bundled Rundeck ResourceModelSource plugin for seamless integration.

## Table of Contents

- [How It Works](#how-it-works)
- [Installation](#installation)
- [Usage](#usage)
- [Filtering](#filtering)
  - [Core Filtering](#core-filtering)
  - [Pattern Matching](#pattern-matching)
  - [Pod Discovery](#pod-discovery)
  - [Output Customization](#output-customization)
- [Node Attributes](#node-attributes)
  - [Workload Nodes](#workload-nodes)
  - [Pod Nodes](#pod-nodes)
  - [Dynamic Attributes](#dynamic-attributes)
- [CLI Reference](#cli-reference)
- [Rundeck Integration](#rundeck-integration)
  - [Script-Based Node Source](#script-based-node-source)
  - [Rundeck Plugin](#rundeck-plugin)
  - [Multi-Cluster Setup](#multi-cluster-setup)
  - [Using Node Attributes in Jobs](#using-node-attributes-in-jobs)
- [Library Usage](#library-usage)
- [Building](#building)
- [Integration Testing](#integration-testing)
- [CI/CD](#cicd)
- [Project Structure](#project-structure)
- [License](#license)

## How It Works

kubectl-rundeck-nodes queries the Kubernetes API for StatefulSets and Deployments, detects Helm release ownership via `app.kubernetes.io/instance` labels, and aggregates multi-workload Helm releases into single nodes. Each discovered workload becomes a Rundeck node with attributes that map to kubectl plugin `--target-*` flags, enabling workload-aware automation in Rundeck jobs.

**Discovery flow:**

1. Lists StatefulSets and Deployments in the target namespace(s)
2. Counts total and healthy (Running) pods for each workload
3. Detects Helm release ownership and aggregates pod counts across workloads belonging to the same release
4. Optionally discovers individual pods with parent workload references
5. Applies filtering (type, label, pattern, health, namespace)
6. Outputs nodes in JSON (default), YAML, or table format

## Installation

### Binary Download

Download from GitHub releases:

```bash
# Linux amd64
curl -LO https://github.com/bluecontainer/kubectl-rundeck-nodes/releases/latest/download/kubectl-rundeck-nodes-linux-amd64
chmod +x kubectl-rundeck-nodes-linux-amd64
sudo mv kubectl-rundeck-nodes-linux-amd64 /usr/local/bin/kubectl-rundeck-nodes
```

### From Source

```bash
go install github.com/bluecontainer/kubectl-rundeck-nodes/cmd/kubectl-rundeck-nodes@latest
```

### Docker

```bash
docker pull bluecontainer/kubectl-rundeck-nodes:latest
```

The image is built on Alpine 3.19, runs as a non-root user (UID 1001), and supports `linux/amd64` and `linux/arm64`.

## Usage

```bash
# Discover workloads in default namespace
kubectl rundeck-nodes

# Discover across all namespaces
kubectl rundeck-nodes -A

# Filter by label
kubectl rundeck-nodes -l app=myapp

# Multi-cluster with custom token suffix
kubectl rundeck-nodes --cluster-name=prod --cluster-token-suffix=clusters/prod/token

# Output as table for human-readable view
kubectl rundeck-nodes -o table

# Direct API server connection (e.g., from within Rundeck)
kubectl-rundeck-nodes --server=https://kubernetes.default.svc --token=$TOKEN -A
```

## Filtering

kubectl-rundeck-nodes provides a multi-phase filtering pipeline to control which workloads appear as Rundeck nodes.

### Core Filtering

```bash
# Only specific workload types
kubectl rundeck-nodes --types=statefulset,helm-release

# Exclude workload types
kubectl rundeck-nodes --exclude-types=deployment

# Exclude by label selector
kubectl rundeck-nodes --exclude-labels=app=operator,tier=control-plane

# Exclude operator controller-manager workloads
kubectl rundeck-nodes --exclude-operator

# Only healthy workloads (all pods running)
kubectl rundeck-nodes --healthy-only

# Only unhealthy workloads (some pods not running)
kubectl rundeck-nodes --unhealthy-only
```

### Pattern Matching

```bash
# Include by name glob pattern
kubectl rundeck-nodes --name-pattern="myapp-*"

# Exclude by name pattern
kubectl rundeck-nodes --exclude-pattern="*-canary"

# Exclude specific namespaces
kubectl rundeck-nodes -A --exclude-namespaces=kube-system,kube-public

# Include only namespaces matching a pattern
kubectl rundeck-nodes -A --namespace-pattern="prod-*"

# Exclude namespaces matching a pattern
kubectl rundeck-nodes -A --exclude-namespace-pattern="test-*"
```

### Pod Discovery

By default, only workload-level nodes are returned. Pod discovery adds individual pods as separate Rundeck nodes, each linked to its parent workload.

```bash
# Include individual pods alongside workload nodes
kubectl rundeck-nodes --include-pods

# Only pods (no workload-level nodes)
kubectl rundeck-nodes --pods-only

# Filter pods by status
kubectl rundeck-nodes --pods-only --pod-status=Running

# Only ready pods
kubectl rundeck-nodes --pods-only --pod-ready-only

# First StatefulSet replica only
kubectl rundeck-nodes --pods-only --pod-name-pattern="*-0"

# Limit pods per workload to avoid node explosion
kubectl rundeck-nodes --include-pods --max-pods-per-workload=5
```

### Output Customization

```bash
# Add custom tags to all nodes
kubectl rundeck-nodes --add-tags=env:prod,team:platform

# Convert Kubernetes labels to Rundeck tags
kubectl rundeck-nodes --labels-as-tags=app.kubernetes.io/name,tier

# Add Kubernetes labels as node attributes
kubectl rundeck-nodes --label-attributes=app.kubernetes.io/version

# Add annotations as node attributes
kubectl rundeck-nodes --annotation-attributes=git.commit/sha
```

## Node Attributes

### Workload Nodes

Each discovered workload becomes a Rundeck node with these attributes:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `nodename` | Unique node identifier | `myapp@production` or `prod:myapp@production` |
| `targetType` | `helm-release`, `statefulset`, or `deployment` | `helm-release` |
| `targetValue` | Workload or release name | `my-release` |
| `targetNamespace` | Workload's namespace | `production` |
| `workloadKind` | `StatefulSet` or `Deployment` | `StatefulSet` |
| `workloadName` | Underlying Kubernetes workload name | `my-release-db` |
| `podCount` | Total pod/replica count | `3` |
| `healthyPods` | Running pod count | `3` |
| `healthy` | Whether all pods are running | `true` |
| `cluster` | Cluster identifier (if `--cluster-name` set) | `prod` |
| `clusterUrl` | Kubernetes API URL | `https://prod.k8s.example.com` |
| `clusterTokenSuffix` | Rundeck Key Storage path suffix | `clusters/prod/token` |

### Pod Nodes

When `--include-pods` or `--pods-only` is used, pod nodes include additional attributes:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `targetType` | Always `pod` | `pod` |
| `targetValue` | Pod name | `my-release-db-0` |
| `podIP` | Pod's cluster IP | `10.244.0.5` |
| `hostIP` | Node IP where pod runs | `192.168.1.10` |
| `k8sNode` | Kubernetes node name | `worker-1` |
| `phase` | Pod phase | `Running` |
| `ready` | All containers ready | `true` |
| `restarts` | Total container restart count | `0` |
| `containerCount` | Number of containers | `2` |
| `readyContainers` | Number of ready containers | `2` |
| `parentType` | Parent workload type | `helm-release` |
| `parentName` | Parent workload name | `my-release` |
| `parentNodename` | Full nodename of parent | `prod:my-release@production` |

### Dynamic Attributes

Labels and annotations can be exposed as node attributes using `--label-attributes` and `--annotation-attributes`. Attribute names are sanitized (dots and slashes become underscores):

```
label_app_kubernetes_io/version: "1.2.3"
annotation_git_commit/sha: "abc123"
```

## CLI Reference

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--kubeconfig` | | | Path to kubeconfig file |
| `--server` | | | Kubernetes API server URL |
| `--token` | | | Bearer token for authentication |
| `--insecure-skip-tls-verify` | | `false` | Skip TLS verification |
| `--namespace` | `-n` | `default` | Namespace to discover |
| `--all-namespaces` | `-A` | `false` | Discover in all namespaces |
| `--selector` | `-l` | | Label selector |
| `--cluster-name` | | | Cluster identifier |
| `--cluster-url` | | | API URL for node attributes |
| `--cluster-token-suffix` | | | Key Storage path suffix |
| `--default-token-suffix` | | `rundeck/k8s-token` | Default path suffix |
| `--output` | `-o` | `json` | Output format: `json`, `yaml`, `table` |
| `--types` | | | Include only these types (comma-separated) |
| `--exclude-types` | | | Exclude these types |
| `--exclude-labels` | | | Exclude by label selectors |
| `--exclude-operator` | | `false` | Exclude operator workloads |
| `--healthy-only` | | `false` | Only healthy workloads |
| `--unhealthy-only` | | `false` | Only unhealthy workloads |
| `--name-pattern` | | | Include name glob patterns |
| `--exclude-pattern` | | | Exclude name glob patterns |
| `--exclude-namespaces` | | | Exclude these namespaces |
| `--namespace-pattern` | | | Include namespace patterns |
| `--exclude-namespace-pattern` | | | Exclude namespace patterns |
| `--add-tags` | | | Custom tags for all nodes |
| `--labels-as-tags` | | | Label keys to convert to tags |
| `--label-attributes` | | | Label keys to add as attributes |
| `--annotation-attributes` | | | Annotation keys to add as attributes |
| `--include-pods` | | `false` | Include individual pod nodes |
| `--pods-only` | | `false` | Only pod nodes (implies `--include-pods`) |
| `--pod-status` | | | Filter pods by phase |
| `--pod-name-pattern` | | | Filter pods by name glob |
| `--pod-ready-only` | | `false` | Only ready pods |
| `--max-pods-per-workload` | | `0` | Max pod nodes per workload (0=unlimited) |

## Rundeck Integration

### Script-Based Node Source

Use kubectl-rundeck-nodes directly as a script-based ResourceModelSource:

```properties
resources.source.1.type=script
resources.source.1.config.file=kubectl-rundeck-nodes
resources.source.1.config.args=-A --server=https://kubernetes.default.svc --token=$TOKEN
resources.source.1.config.format=resourcejson
```

### Rundeck Plugin

The bundled [rundeck-k8s-nodes](rundeck-plugin/) plugin provides a native Rundeck ResourceModelSource with a configuration UI. It supports three execution modes:

| Mode | Description |
|------|-------------|
| **native** | Runs the binary directly on the Rundeck server (default) |
| **docker** | Runs in a Docker container on the Rundeck host |
| **kubernetes** | Runs as an ephemeral pod in the target cluster |

Install the plugin by copying `rundeck-k8s-nodes-1.0.0.zip` to Rundeck's `libext` directory.

See the [plugin README](rundeck-plugin/README.md) for full configuration details.

### Multi-Cluster Setup

Configure multiple node sources with different cluster identifiers to manage several clusters from a single Rundeck instance:

```properties
# Production cluster
resources.source.1.type=k8s-workload-nodes
resources.source.1.config.k8s_url=https://prod.k8s.example.com
resources.source.1.config.k8s_token=keys/clusters/prod/token
resources.source.1.config.cluster_name=prod
resources.source.1.config.cluster_token_suffix=clusters/prod/token

# Staging cluster
resources.source.2.type=k8s-workload-nodes
resources.source.2.config.k8s_url=https://staging.k8s.example.com
resources.source.2.config.k8s_token=keys/clusters/staging/token
resources.source.2.config.cluster_name=staging
resources.source.2.config.cluster_token_suffix=clusters/staging/token
```

Jobs use `@node.clusterTokenSuffix@` to dynamically select credentials per node:

```bash
_TOKEN_SUFFIX="@node.clusterTokenSuffix@"
CLUSTER_TOKEN="$(cat /path/to/keys/${_TOKEN_SUFFIX})"
```

### Using Node Attributes in Jobs

Rundeck jobs can reference node attributes for workload-aware targeting:

```bash
_NODE_TYPE="@node.targetType@"
_NODE_VALUE="@node.targetValue@"
_NODE_NS="@node.targetNamespace@"

case "$_NODE_TYPE" in
  helm-release) kubectl myapp --target-helm-release="$_NODE_VALUE" -n "$_NODE_NS" ;;
  statefulset)  kubectl myapp --target-statefulset="$_NODE_VALUE" -n "$_NODE_NS" ;;
  deployment)   kubectl myapp --target-deployment="$_NODE_VALUE" -n "$_NODE_NS" ;;
esac
```

## Library Usage

The `pkg/nodes` package can be imported for custom integrations:

```go
import "github.com/bluecontainer/kubectl-rundeck-nodes/pkg/nodes"

opts := nodes.DiscoverOptions{
    Namespace:     "production",
    AllNamespaces: false,
    ClusterName:   "prod",
    HealthyOnly:   true,
    IncludePods:   true,
    MaxPodsPerWorkload: 3,
}

discovered, err := nodes.Discover(ctx, dynamicClient, opts)
if err != nil {
    return err
}

nodes.Write(os.Stdout, discovered, nodes.FormatJSON)
```

## Building

### Binary

```bash
make build              # Build for local platform → bin/kubectl-rundeck-nodes
make cross-compile      # Build for Linux, macOS, Windows (amd64 + arm64)
make install            # Install to $GOPATH/bin
```

### Docker Image

```bash
make docker-build       # Single-platform local image
make docker-buildx      # Multi-arch image (linux/amd64, linux/arm64) with push
```

### Rundeck Plugin

```bash
cd rundeck-plugin
make build              # Build plugin ZIP → rundeck-k8s-nodes-1.0.0.zip
```

### Testing

```bash
make test               # Run all tests
make test-coverage      # Generate coverage report → coverage.html
```

## Integration Testing

The `integration-test/` directory contains an end-to-end test that verifies the Rundeck plugin discovers Kubernetes workloads correctly. It spins up a real k3s cluster and Rundeck instance via Docker Compose, loads the plugin, and asserts the resource model output.

### Prerequisites

- Docker with [Compose V2](https://docs.docker.com/compose/) (`docker compose`)
- Go toolchain (to build the plugin binary)
- `jq` and `make`

### What It Does

1. Builds the Rundeck plugin ZIP for the Docker host architecture
2. Starts a **k3s** cluster and **Rundeck** server via Docker Compose
3. Deploys an **nginx Deployment with 3 replicas** and creates a read-only ServiceAccount for the plugin
4. Stores the ServiceAccount token in Rundeck Key Storage
5. Creates a Rundeck project (`k8s-integration-test`) configured with the `k8s-workload-nodes` resource model source
6. Queries the Rundeck API and verifies the plugin discovered the nginx deployment with correct attributes (`targetType=deployment`, `podCount=3`, `healthyPods=3`, etc.)

### Running

```bash
# From the repo root
make integration-test

# Or from the integration-test/ directory
make test
```

To keep containers running after a failure for debugging:

```bash
SKIP_CLEANUP=1 integration-test/scripts/run-test.sh
```

Additional targets inside `integration-test/`:

```bash
make logs      # View container logs
make status    # Show container status
make clean     # Tear down containers and volumes
```

## CI/CD

### CI (Pull Requests & Main)

Every push to `main` and every pull request runs four parallel checks:

| Job | Description |
|-----|-------------|
| **Lint** | Checks `gofmt` formatting and `go vet` |
| **Test** | Runs `make test-coverage`; uploads coverage HTML as an artifact on PRs |
| **Build** | Cross-compiles all binaries and builds the Rundeck plugin ZIP |
| **Docker** | Verifies multi-arch Docker image builds (no push) |

### Release (Tag Push)

Pushing a tag matching `v*` (e.g., `git tag v1.0.0 && git push origin v1.0.0`) triggers the release pipeline:

1. **Build & Release** — runs tests, cross-compiles binaries for 5 platforms (linux/darwin/windows, amd64/arm64), builds Rundeck plugin ZIPs for linux-amd64 and linux-arm64, and creates a GitHub Release with all artifacts attached.
2. **Docker Push** — builds a multi-arch image (linux/amd64, linux/arm64) and pushes to both Docker Hub (`bluecontainer/kubectl-rundeck-nodes`) and GHCR (`ghcr.io/bluecontainer/kubectl-rundeck-nodes`) with semver tags (`1.2.3`, `1.2`, `1`, `latest`).

**Required repository secrets for Docker Hub:**
- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

GHCR authentication uses the built-in `GITHUB_TOKEN` automatically.

## Project Structure

```
├── cmd/kubectl-rundeck-nodes/   CLI entry point (cobra)
│   └── main.go
├── pkg/nodes/                   Core discovery library
│   ├── types.go                 Data structures and constants
│   ├── discover.go              Workload discovery logic
│   ├── filters.go               Multi-phase filtering pipeline
│   ├── attributes.go            Tag and attribute building
│   ├── output.go                JSON/YAML/table output formatting
│   └── *_test.go                Unit tests
├── rundeck-plugin/              Rundeck ResourceModelSource plugin
│   ├── plugin.yaml              Plugin manifest
│   ├── contents/nodes.sh        Bash wrapper for Rundeck
│   ├── Makefile                 Plugin build
│   └── README.md                Plugin documentation
├── integration-test/            End-to-end integration test
│   ├── docker-compose.yml       k3s + Rundeck services
│   ├── k8s/                     Kubernetes manifests (nginx, RBAC)
│   ├── rundeck/                 Rundeck project config and API tokens
│   ├── scripts/run-test.sh      Test orchestration script
│   └── Makefile                 Test convenience targets
├── Dockerfile                   Multi-arch container image
├── Makefile                     Build, test, cross-compile targets
└── go.mod                       Go 1.25, client-go v0.32.0
```

## License

Apache-2.0
