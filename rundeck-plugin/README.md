# rundeck-k8s-nodes

A Rundeck ResourceModelSource plugin that discovers Kubernetes workloads (Helm releases, StatefulSets, Deployments) as Rundeck nodes.

## Installation

### From ZIP

1. Download `rundeck-k8s-nodes-1.0.0.zip` from releases
2. Copy to Rundeck's `libext` directory
3. Restart Rundeck

### Build from Source

```bash
make build
# Copy rundeck-k8s-nodes-1.0.0.zip to $RUNDECK_HOME/libext/
```

## Prerequisites

The `kubectl-rundeck-nodes` binary must be available:

**Native mode**: Install on the Rundeck server
```bash
curl -LO https://github.com/bluecontainer/kubectl-rundeck-nodes/releases/latest/download/kubectl-rundeck-nodes-linux-amd64
chmod +x kubectl-rundeck-nodes-linux-amd64
sudo mv kubectl-rundeck-nodes-linux-amd64 /usr/local/bin/kubectl-rundeck-nodes
```

**Docker mode**: Pull the image
```bash
docker pull bluecontainer/kubectl-rundeck-nodes:latest
```

## Configuration

Add a node source to your Rundeck project:

1. Go to Project Settings → Edit Nodes
2. Add Source → "Kubernetes Workload Nodes"
3. Configure:

| Setting | Description |
|---------|-------------|
| Kubernetes Token | ServiceAccount token (select from Key Storage) |
| Kubernetes API URL | API server URL (e.g., `https://kubernetes.default.svc`) |
| Namespace | Namespace to discover (empty = all namespaces) |
| Label Selector | Filter workloads by labels |
| Execution Mode | `native` or `docker` |
| Docker Image | Image for docker mode |
| Cluster Name | Identifier for multi-cluster |
| Cluster Token Suffix | Key Storage path suffix |

## Node Attributes

Discovered nodes have these attributes:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `targetType` | Workload type | `helm-release` |
| `targetValue` | Workload name | `my-release` |
| `targetNamespace` | Namespace | `production` |
| `workloadKind` | K8s kind | `StatefulSet` |
| `workloadName` | K8s name | `my-release-db` |
| `podCount` | Total replicas | `3` |
| `healthyPods` | Running pods | `3` |
| `cluster` | Cluster name | `prod` |
| `clusterUrl` | API URL | `https://...` |
| `clusterTokenSuffix` | Token path | `clusters/prod/token` |

## Multi-Cluster Setup

For multi-cluster deployments, add multiple node sources with different cluster configurations:

```properties
# Cluster 1: Production
resources.source.1.type=k8s-workload-nodes
resources.source.1.config.k8s_token=keys/clusters/prod/token
resources.source.1.config.k8s_url=https://prod.k8s.example.com
resources.source.1.config.cluster_name=prod
resources.source.1.config.cluster_token_suffix=clusters/prod/token

# Cluster 2: Staging
resources.source.2.type=k8s-workload-nodes
resources.source.2.config.k8s_token=keys/clusters/staging/token
resources.source.2.config.k8s_url=https://staging.k8s.example.com
resources.source.2.config.cluster_name=staging
resources.source.2.config.cluster_token_suffix=clusters/staging/token
```

Jobs can use `@node.clusterTokenSuffix@` to dynamically select credentials per node.

## Using with Jobs

Jobs can use node attributes for targeting:

```bash
# In job script
_NODE_TYPE="@node.targetType@"
_NODE_VALUE="@node.targetValue@"

case "$_NODE_TYPE" in
  helm-release) kubectl myapp --target-helm-release="$_NODE_VALUE" ;;
  statefulset)  kubectl myapp --target-statefulset="$_NODE_VALUE" ;;
  deployment)   kubectl myapp --target-deployment="$_NODE_VALUE" ;;
esac
```
