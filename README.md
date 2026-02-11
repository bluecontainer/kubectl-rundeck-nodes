# kubectl-rundeck-nodes

A kubectl plugin that discovers Kubernetes workloads (Helm releases, StatefulSets, Deployments) and outputs them as Rundeck resource model JSON for use as a script-based node source.

## Installation

### Binary Download

Download from GitHub releases:

```bash
# Linux
curl -LO https://github.com/bluecontainer/kubectl-rundeck-nodes/releases/latest/download/kubectl-rundeck-nodes-linux-amd64
chmod +x kubectl-rundeck-nodes-linux-amd64
sudo mv kubectl-rundeck-nodes-linux-amd64 /usr/local/bin/kubectl-rundeck-nodes
```

### From Source

```bash
go install github.com/bluecontainer/kubectl-rundeck-nodes/cmd/kubectl-rundeck-nodes@latest
```

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

# Output as table
kubectl rundeck-nodes -o table
```

## Node Attributes

Each discovered workload becomes a Rundeck node with these attributes:

| Attribute | Description |
|-----------|-------------|
| `targetType` | `helm-release`, `statefulset`, or `deployment` |
| `targetValue` | Workload or release name |
| `targetNamespace` | Workload's namespace |
| `workloadKind` | `StatefulSet` or `Deployment` |
| `workloadName` | Underlying workload name |
| `podCount` | Total pod count |
| `healthyPods` | Running pod count |
| `cluster` | Cluster identifier (if set) |
| `clusterUrl` | Kubernetes API URL |
| `clusterTokenSuffix` | Key Storage path suffix |

## Rundeck Integration

Use this as a script-based ResourceModelSource in Rundeck:

```properties
resources.source.1.type=script
resources.source.1.config.file=kubectl-rundeck-nodes
resources.source.1.config.args=-A --server=https://kubernetes --token=$TOKEN
resources.source.1.config.format=resourcejson
```

Or use the bundled [rundeck-k8s-nodes](../rundeck-k8s-nodes/) Rundeck plugin.

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--kubeconfig` | | Path to kubeconfig file |
| `--server` | | Kubernetes API server URL |
| `--token` | | Bearer token for authentication |
| `--insecure-skip-tls-verify` | | Skip TLS verification |
| `--namespace` | `-n` | Namespace to discover |
| `--all-namespaces` | `-A` | Discover in all namespaces |
| `--selector` | `-l` | Label selector |
| `--cluster-name` | | Cluster identifier |
| `--cluster-url` | | API URL for node attributes |
| `--cluster-token-suffix` | | Key Storage path suffix |
| `--default-token-suffix` | | Default path suffix |
| `--output` | `-o` | Output format: json, yaml, table |

## Library Usage

The `pkg/nodes` package can be imported for custom integrations:

```go
import "github.com/bluecontainer/kubectl-rundeck-nodes/pkg/nodes"

opts := nodes.DiscoverOptions{
    Namespace:     "production",
    AllNamespaces: false,
    ClusterName:   "prod",
}

discovered, err := nodes.Discover(ctx, dynamicClient, opts)
if err != nil {
    return err
}

nodes.Write(os.Stdout, discovered, nodes.FormatJSON)
```
