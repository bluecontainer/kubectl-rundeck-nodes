#!/bin/bash
# Integration test for the kubectl-rundeck-nodes Rundeck plugin.
#
# This script:
#   1. Builds the Rundeck plugin ZIP
#   2. Starts k3s + Rundeck via docker compose
#   3. Deploys nginx (3 replicas) and RBAC into k3s
#   4. Configures Rundeck with the plugin as a resource model source
#   5. Verifies the plugin discovers the nginx deployment correctly
#
# Prerequisites: docker, docker compose (v2), jq, make, go

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$PROJECT_DIR/.." && pwd)"

COMPOSE_PROJECT="kubectl-rundeck-nodes-test"
RUNDECK_URL="http://localhost:4440"
API_TOKEN=""
API_VERSION=41
COOKIES_FILE=$(mktemp)

# --- Logging helpers ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()   { echo -e "${GREEN}[TEST]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[FAIL]${NC} $*"; }
info()  { echo -e "${CYAN}[INFO]${NC} $*"; }

# --- Cleanup on exit ---
cleanup() {
    local exit_code=$?
    if [ "${SKIP_CLEANUP:-}" = "1" ]; then
        warn "SKIP_CLEANUP=1 — leaving containers running for debugging"
        warn "Run: docker compose -p $COMPOSE_PROJECT -f $PROJECT_DIR/docker-compose.yml down -v"
        return
    fi
    log "Cleaning up..."
    rm -f "$COOKIES_FILE"
    rm -rf "$PROJECT_DIR/plugins"
    docker compose -p "$COMPOSE_PROJECT" -f "$PROJECT_DIR/docker-compose.yml" down -v 2>/dev/null || true
    exit $exit_code
}
trap cleanup EXIT

# --- Helper: run kubectl inside k3s container ---
k3s_kubectl() {
    docker compose -p "$COMPOSE_PROJECT" -f "$PROJECT_DIR/docker-compose.yml" \
        exec -T k3s kubectl "$@"
}

# --- Helper: Rundeck API call ---
rundeck_api() {
    local method="$1"
    local path="$2"
    shift 2
    curl -sf -X "$method" \
        "${RUNDECK_URL}/api/${API_VERSION}${path}" \
        -H "X-Rundeck-Auth-Token: ${API_TOKEN}" \
        -H "Accept: application/json" \
        "$@"
}

# --- Helper: wait with retries ---
wait_for() {
    local description="$1"
    local max_attempts="$2"
    local interval="$3"
    shift 3

    info "Waiting for ${description}..."
    for i in $(seq 1 "$max_attempts"); do
        if "$@" 2>/dev/null; then
            log "${description} — ready"
            return 0
        fi
        if [ "$i" -eq "$max_attempts" ]; then
            error "${description} — timed out after $((max_attempts * interval))s"
            return 1
        fi
        sleep "$interval"
    done
}

# ============================================================
#  Step 1: Build the Rundeck plugin
# ============================================================
log "=== Step 1: Building Rundeck plugin ==="

# Detect Docker platform architecture for cross-compilation
DOCKER_ARCH=$(docker info --format '{{.Architecture}}' 2>/dev/null || echo "x86_64")
case "$DOCKER_ARCH" in
    x86_64|amd64)  GOARCH=amd64 ;;
    aarch64|arm64) GOARCH=arm64 ;;
    *)
        error "Unsupported Docker architecture: $DOCKER_ARCH"
        exit 1
        ;;
esac
info "Building plugin for linux/${GOARCH} (Docker arch: ${DOCKER_ARCH})"

make -C "$REPO_ROOT/rundeck-plugin" clean build GOOS=linux GOARCH="$GOARCH"

# Stage the plugin ZIP into a local directory for the Docker volume mount
mkdir -p "$PROJECT_DIR/plugins"
cp "$REPO_ROOT/rundeck-plugin/rundeck-k8s-nodes-1.0.0.zip" "$PROJECT_DIR/plugins/"
log "Plugin staged: integration-test/plugins/rundeck-k8s-nodes-1.0.0.zip"

# ============================================================
#  Step 2: Start docker compose
# ============================================================
log "=== Step 2: Starting docker compose ==="
docker compose -p "$COMPOSE_PROJECT" -f "$PROJECT_DIR/docker-compose.yml" up -d

# ============================================================
#  Step 3: Wait for k3s
# ============================================================
log "=== Step 3: Waiting for k3s ==="
wait_for "k3s cluster" 60 2 \
    docker compose -p "$COMPOSE_PROJECT" -f "$PROJECT_DIR/docker-compose.yml" \
        exec -T k3s kubectl get nodes --no-headers

# ============================================================
#  Step 4: Deploy RBAC and nginx
# ============================================================
log "=== Step 4: Deploying RBAC and nginx ==="

k3s_kubectl apply -f - < "$PROJECT_DIR/k8s/rbac.yaml"
log "RBAC created (ServiceAccount: rundeck-reader)"

k3s_kubectl apply -f - < "$PROJECT_DIR/k8s/nginx-deployment.yaml"
log "nginx Deployment applied (3 replicas)"

# Wait for nginx pods to be ready
wait_for "nginx pods (3/3 ready)" 60 3 \
    bash -c "[ \"\$(docker compose -p $COMPOSE_PROJECT -f $PROJECT_DIR/docker-compose.yml exec -T k3s kubectl get deployment nginx -o jsonpath='{.status.readyReplicas}' 2>/dev/null)\" = '3' ]"

# Show deployment status
k3s_kubectl get deployment nginx
k3s_kubectl get pods -l app=nginx

# ============================================================
#  Step 5: Extract ServiceAccount token
# ============================================================
log "=== Step 5: Extracting ServiceAccount token ==="

# Wait for the token secret to be populated by the token controller
wait_for "ServiceAccount token secret" 30 2 \
    bash -c "docker compose -p $COMPOSE_PROJECT -f $PROJECT_DIR/docker-compose.yml exec -T k3s kubectl get secret rundeck-reader-token -o jsonpath='{.data.token}' 2>/dev/null | grep -q '.'"

K8S_TOKEN=$(k3s_kubectl get secret rundeck-reader-token -o jsonpath='{.data.token}' | base64 -d)

if [ -z "$K8S_TOKEN" ]; then
    error "Failed to extract ServiceAccount token"
    exit 1
fi
log "ServiceAccount token extracted (${#K8S_TOKEN} chars)"

# ============================================================
#  Step 6: Wait for Rundeck and authenticate
# ============================================================
log "=== Step 6: Waiting for Rundeck ==="

# Rundeck's login page comes up before the API is ready (returns 503).
# Retry the full login + token flow until the API is fully available.
info "Waiting for Rundeck API and authenticating..."
for i in $(seq 1 60); do
    # Establish session
    curl -s -c "$COOKIES_FILE" -o /dev/null "${RUNDECK_URL}/user/login" 2>/dev/null || { sleep 3; continue; }

    # Login
    curl -s -c "$COOKIES_FILE" -b "$COOKIES_FILE" -o /dev/null -L \
        -d "j_username=admin&j_password=admin" \
        "${RUNDECK_URL}/j_security_check" 2>/dev/null || { sleep 3; continue; }

    # Check API is ready (not 503) and session is authenticated (200)
    LOGIN_CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIES_FILE" \
        "${RUNDECK_URL}/api/${API_VERSION}/system/info" \
        -H "Accept: application/json" 2>/dev/null)
    if [ "$LOGIN_CODE" = "200" ]; then
        log "Rundeck API ready and session authenticated"
        break
    fi

    if [ "$i" -eq 60 ]; then
        error "Rundeck failed to become ready (last HTTP ${LOGIN_CODE})"
        exit 1
    fi
    sleep 3
done

# Generate an API token via session auth
info "Generating API token..."
TOKEN_RESPONSE=$(curl -s -b "$COOKIES_FILE" \
    -X POST "${RUNDECK_URL}/api/${API_VERSION}/tokens" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -d '{"user":"admin","roles":"admin","duration":"1h"}')
API_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.token // empty')

if [ -z "$API_TOKEN" ]; then
    error "Failed to generate Rundeck API token"
    error "Response: ${TOKEN_RESPONSE}"
    exit 1
fi
log "API token generated"

# Verify plugin is loaded
info "Checking installed plugins..."
PLUGIN_LIST=$(rundeck_api GET /plugin/list)
if echo "$PLUGIN_LIST" | jq -e '.[] | select(.name == "k8s-workload-nodes" and .service == "ResourceModelSource")' > /dev/null 2>&1; then
    log "Plugin 'k8s-workload-nodes' loaded"
else
    error "Plugin 'k8s-workload-nodes' NOT found in Rundeck"
    info "Loaded ResourceModelSource plugins:"
    echo "$PLUGIN_LIST" | jq '[.[] | select(.service == "ResourceModelSource") | .name]'
    info "Rundeck libext contents:"
    docker compose -p "$COMPOSE_PROJECT" -f "$PROJECT_DIR/docker-compose.yml" \
        exec -T rundeck ls -la /home/rundeck/libext/ 2>/dev/null || true
    exit 1
fi

# ============================================================
#  Step 7: Store K8s token in Rundeck Key Storage
# ============================================================
log "=== Step 7: Storing K8s token in Rundeck Key Storage ==="

curl -sf -X POST \
    "${RUNDECK_URL}/api/${API_VERSION}/storage/keys/integration-test/k8s-token" \
    -H "X-Rundeck-Auth-Token: ${API_TOKEN}" \
    -H "Content-Type: application/x-rundeck-data-password" \
    -d "$K8S_TOKEN" > /dev/null

log "Token stored at keys/integration-test/k8s-token"

# ============================================================
#  Step 8: Create Rundeck project
# ============================================================
log "=== Step 8: Creating Rundeck project ==="

rundeck_api POST /projects \
    -H "Content-Type: application/json" \
    -d @"$PROJECT_DIR/rundeck/project.json" > /dev/null

log "Project 'k8s-integration-test' created"

# ============================================================
#  Step 9: Wait for resource model and query
# ============================================================
log "=== Step 9: Querying resource model ==="

# Give Rundeck time to refresh the resource model from the plugin
RESOURCES=""
wait_for "resource model to contain nginx" 20 5 \
    bash -c "
        RESOURCES=\$(curl -sf \
            '${RUNDECK_URL}/api/${API_VERSION}/project/k8s-integration-test/resources' \
            -H 'X-Rundeck-Auth-Token: ${API_TOKEN}' \
            -H 'Accept: application/json' 2>/dev/null)
        echo \"\$RESOURCES\" | jq -e 'keys[] | select(contains(\"nginx\"))' > /dev/null 2>&1
    "

# Fetch final resource model
RESOURCES=$(rundeck_api GET /project/k8s-integration-test/resources)

info "Resource model:"
echo "$RESOURCES" | jq .

# ============================================================
#  Step 10: Assertions
# ============================================================
log "=== Step 10: Verifying assertions ==="

# Find the nginx node key
NODE_KEY=$(echo "$RESOURCES" | jq -r 'keys[] | select(contains("nginx"))' | head -1)
if [ -z "$NODE_KEY" ]; then
    error "No nginx node found in resource model"
    echo "$RESOURCES" | jq .
    exit 1
fi
log "Found nginx node: ${NODE_KEY}"

PASSED=true
assert() {
    local field="$1"
    local expected="$2"
    local actual
    actual=$(echo "$RESOURCES" | jq -r ".[\"$NODE_KEY\"].$field // empty")
    if [ "$actual" = "$expected" ]; then
        log "  PASS  $field = $expected"
    else
        error "  FAIL  $field = '$actual' (expected '$expected')"
        PASSED=false
    fi
}

assert "targetType"      "deployment"
assert "targetValue"     "nginx"
assert "targetNamespace" "default"
assert "podCount"        "3"
assert "healthyPods"     "3"
assert "healthy"         "true"

echo ""
if [ "$PASSED" = true ]; then
    log "========================================="
    log "  All assertions passed!"
    log "========================================="
else
    error "========================================="
    error "  Some assertions failed"
    error "========================================="
    exit 1
fi
