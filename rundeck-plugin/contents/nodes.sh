#!/bin/bash
# Rundeck ResourceModelSource script for Kubernetes workload discovery
# Invokes kubectl-rundeck-nodes to discover StatefulSets, Deployments, and Helm releases

set -euo pipefail

# Configuration from Rundeck plugin (passed via RD_CONFIG_* env vars)
K8S_TOKEN="${RD_CONFIG_K8S_TOKEN:-}"
K8S_URL="${RD_CONFIG_K8S_URL:-}"
NAMESPACE="${RD_CONFIG_NAMESPACE:-}"
LABEL_SELECTOR="${RD_CONFIG_LABEL_SELECTOR:-}"
EXECUTION_MODE="${RD_CONFIG_EXECUTION_MODE:-native}"
DOCKER_IMAGE="${RD_CONFIG_DOCKER_IMAGE:-ghcr.io/bluecontainer/kubectl-rundeck-nodes:latest}"
DOCKER_NETWORK="${RD_CONFIG_DOCKER_NETWORK:-host}"
PLUGIN_NAMESPACE="${RD_CONFIG_PLUGIN_NAMESPACE:-default}"
SERVICE_ACCOUNT="${RD_CONFIG_SERVICE_ACCOUNT:-default}"
IMAGE_PULL_POLICY="${RD_CONFIG_IMAGE_PULL_POLICY:-IfNotPresent}"
CLUSTER_NAME="${RD_CONFIG_CLUSTER_NAME:-}"
CLUSTER_TOKEN_SUFFIX="${RD_CONFIG_CLUSTER_TOKEN_SUFFIX:-}"
DEFAULT_TOKEN_SUFFIX="${RD_CONFIG_DEFAULT_TOKEN_SUFFIX:-rundeck/k8s-token}"

# Phase 1: Core Filtering Options
TYPES="${RD_CONFIG_TYPES:-}"
EXCLUDE_TYPES="${RD_CONFIG_EXCLUDE_TYPES:-}"
EXCLUDE_LABELS="${RD_CONFIG_EXCLUDE_LABELS:-}"
EXCLUDE_OPERATOR="${RD_CONFIG_EXCLUDE_OPERATOR:-true}"
HEALTHY_ONLY="${RD_CONFIG_HEALTHY_ONLY:-false}"
UNHEALTHY_ONLY="${RD_CONFIG_UNHEALTHY_ONLY:-false}"

# Phase 2: Pattern Matching Options
NAME_PATTERN="${RD_CONFIG_NAME_PATTERN:-}"
EXCLUDE_PATTERN="${RD_CONFIG_EXCLUDE_PATTERN:-}"
EXCLUDE_NAMESPACES="${RD_CONFIG_EXCLUDE_NAMESPACES:-}"
NAMESPACE_PATTERN="${RD_CONFIG_NAMESPACE_PATTERN:-}"
EXCLUDE_NAMESPACE_PATTERN="${RD_CONFIG_EXCLUDE_NAMESPACE_PATTERN:-}"

# Phase 4: Output Customization Options
ADD_TAGS="${RD_CONFIG_ADD_TAGS:-}"
LABELS_AS_TAGS="${RD_CONFIG_LABELS_AS_TAGS:-}"
LABEL_ATTRIBUTES="${RD_CONFIG_LABEL_ATTRIBUTES:-}"
ANNOTATION_ATTRIBUTES="${RD_CONFIG_ANNOTATION_ATTRIBUTES:-}"

# Phase 5: Pod Discovery Options
INCLUDE_PODS="${RD_CONFIG_INCLUDE_PODS:-false}"
PODS_ONLY="${RD_CONFIG_PODS_ONLY:-false}"
POD_STATUS="${RD_CONFIG_POD_STATUS:-}"
POD_NAME_PATTERN="${RD_CONFIG_POD_NAME_PATTERN:-}"
POD_READY_ONLY="${RD_CONFIG_POD_READY_ONLY:-false}"
MAX_PODS_PER_WORKLOAD="${RD_CONFIG_MAX_PODS_PER_WORKLOAD:-0}"

if [ -z "$K8S_TOKEN" ]; then
  echo "Error: K8S_TOKEN not provided" >&2
  exit 1
fi

if [ -z "$K8S_URL" ]; then
  echo "Error: K8S_URL not provided" >&2
  exit 1
fi

# Build flags
FLAGS=""
if [ -n "$NAMESPACE" ]; then
  FLAGS="$FLAGS -n $NAMESPACE"
else
  FLAGS="$FLAGS -A"
fi
[ -n "$LABEL_SELECTOR" ] && FLAGS="$FLAGS -l $LABEL_SELECTOR"
[ -n "$CLUSTER_NAME" ] && FLAGS="$FLAGS --cluster-name=$CLUSTER_NAME"
[ -n "$K8S_URL" ] && FLAGS="$FLAGS --cluster-url=$K8S_URL"
[ -n "$CLUSTER_TOKEN_SUFFIX" ] && FLAGS="$FLAGS --cluster-token-suffix=$CLUSTER_TOKEN_SUFFIX"
[ -n "$DEFAULT_TOKEN_SUFFIX" ] && FLAGS="$FLAGS --default-token-suffix=$DEFAULT_TOKEN_SUFFIX"

# Phase 1: Core Filtering flags
[ -n "$TYPES" ] && FLAGS="$FLAGS --types=$TYPES"
[ -n "$EXCLUDE_TYPES" ] && FLAGS="$FLAGS --exclude-types=$EXCLUDE_TYPES"
[ -n "$EXCLUDE_LABELS" ] && FLAGS="$FLAGS --exclude-labels=$EXCLUDE_LABELS"
[ "$EXCLUDE_OPERATOR" = "true" ] && FLAGS="$FLAGS --exclude-operator"
[ "$HEALTHY_ONLY" = "true" ] && FLAGS="$FLAGS --healthy-only"
[ "$UNHEALTHY_ONLY" = "true" ] && FLAGS="$FLAGS --unhealthy-only"

# Phase 2: Pattern Matching flags
[ -n "$NAME_PATTERN" ] && FLAGS="$FLAGS --name-pattern=$NAME_PATTERN"
[ -n "$EXCLUDE_PATTERN" ] && FLAGS="$FLAGS --exclude-pattern=$EXCLUDE_PATTERN"
[ -n "$EXCLUDE_NAMESPACES" ] && FLAGS="$FLAGS --exclude-namespaces=$EXCLUDE_NAMESPACES"
[ -n "$NAMESPACE_PATTERN" ] && FLAGS="$FLAGS --namespace-pattern=$NAMESPACE_PATTERN"
[ -n "$EXCLUDE_NAMESPACE_PATTERN" ] && FLAGS="$FLAGS --exclude-namespace-pattern=$EXCLUDE_NAMESPACE_PATTERN"

# Phase 4: Output Customization flags
[ -n "$ADD_TAGS" ] && FLAGS="$FLAGS --add-tags=$ADD_TAGS"
[ -n "$LABELS_AS_TAGS" ] && FLAGS="$FLAGS --labels-as-tags=$LABELS_AS_TAGS"
[ -n "$LABEL_ATTRIBUTES" ] && FLAGS="$FLAGS --label-attributes=$LABEL_ATTRIBUTES"
[ -n "$ANNOTATION_ATTRIBUTES" ] && FLAGS="$FLAGS --annotation-attributes=$ANNOTATION_ATTRIBUTES"

# Phase 5: Pod Discovery flags
[ "$INCLUDE_PODS" = "true" ] && FLAGS="$FLAGS --include-pods"
[ "$PODS_ONLY" = "true" ] && FLAGS="$FLAGS --pods-only"
[ -n "$POD_STATUS" ] && FLAGS="$FLAGS --pod-status=$POD_STATUS"
[ -n "$POD_NAME_PATTERN" ] && FLAGS="$FLAGS --pod-name-pattern=$POD_NAME_PATTERN"
[ "$POD_READY_ONLY" = "true" ] && FLAGS="$FLAGS --pod-ready-only"
[ "$MAX_PODS_PER_WORKLOAD" != "0" ] && FLAGS="$FLAGS --max-pods-per-workload=$MAX_PODS_PER_WORKLOAD"

# Find the kubectl-rundeck-nodes binary
# Priority: 1) bundled in plugin, 2) system PATH
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/kubectl-rundeck-nodes" ]; then
  # Ensure execute permission (Java ZIP extraction may strip it)
  [ -x "$SCRIPT_DIR/kubectl-rundeck-nodes" ] || chmod +x "$SCRIPT_DIR/kubectl-rundeck-nodes"
  KUBECTL_RUNDECK_NODES="$SCRIPT_DIR/kubectl-rundeck-nodes"
elif command -v kubectl-rundeck-nodes &>/dev/null; then
  KUBECTL_RUNDECK_NODES="kubectl-rundeck-nodes"
else
  echo "Error: kubectl-rundeck-nodes not found (not bundled or in PATH)" >&2
  exit 1
fi

case "$EXECUTION_MODE" in
  native)
    # Native execution: kubectl-rundeck-nodes runs directly on Rundeck host
    "$KUBECTL_RUNDECK_NODES" --server="$K8S_URL" --token="$K8S_TOKEN" \
      --insecure-skip-tls-verify $FLAGS
    ;;

  docker)
    # Docker execution: kubectl-rundeck-nodes runs in Docker container
    docker run --rm --network "$DOCKER_NETWORK" "$DOCKER_IMAGE" \
      --server="$K8S_URL" --token="$K8S_TOKEN" \
      --insecure-skip-tls-verify $FLAGS
    ;;

  kubernetes)
    # Kubernetes execution: kubectl-rundeck-nodes runs in ephemeral K8s pod
    POD="rundeck-nodes-$(date +%s)-$RANDOM"
    kubectl --server="$K8S_URL" --token="$K8S_TOKEN" --insecure-skip-tls-verify \
      run "$POD" --image="$DOCKER_IMAGE" --restart=Never --rm -i --quiet \
      --image-pull-policy="$IMAGE_PULL_POLICY" \
      -n "$PLUGIN_NAMESPACE" \
      --overrides='{"spec":{"serviceAccountName":"'"$SERVICE_ACCOUNT"'"}}' \
      -- $FLAGS
    ;;

  *)
    echo "Error: Unknown execution mode: $EXECUTION_MODE" >&2
    exit 1
    ;;
esac
