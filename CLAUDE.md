# CLAUDE.md — Project Guide for AI Assistants

## What This Project Is

`kubectl-rundeck-nodes` is a Go tool that discovers Kubernetes workloads (Helm releases, StatefulSets, Deployments, Pods) and outputs them as Rundeck resource model JSON. It ships as:

1. **CLI / kubectl plugin** — standalone binary, usable as `kubectl rundeck-nodes`
2. **Go library** — `pkg/nodes` package for programmatic use
3. **Rundeck plugin** — ResourceModelSource ZIP wrapping the CLI binary
4. **Docker image** — multi-arch container on GHCR

Repository: `github.com/bluecontainer/kubectl-rundeck-nodes`

## Project Structure

```
cmd/kubectl-rundeck-nodes/main.go   # CLI entry point (Cobra-based, ~290 lines)
pkg/nodes/                          # Core library
  types.go                          # Data structures, constants
  discover.go                       # K8s API discovery logic
  filters.go                        # 5-phase filtering pipeline
  attributes.go                     # Tag/attribute building, sanitization
  output.go                         # JSON/YAML/table serialization
  *_test.go                         # Unit tests for each module
rundeck-plugin/
  plugin.yaml                       # Rundeck plugin manifest (50+ config options)
  contents/nodes.sh                 # Bash wrapper translating config → CLI flags
  Makefile                          # Plugin ZIP build (linux/amd64 + arm64)
integration-test/
  docker-compose.yml                # k3s + Rundeck test environment
  scripts/run-test.sh               # E2E test orchestration
  k8s/                              # Test manifests (RBAC, nginx deployment)
Makefile                            # Top-level build system
Dockerfile                          # Multi-arch Alpine container
.github/workflows/
  ci.yaml                           # Lint → Test → Build → Docker (on push/PR)
  release.yaml                      # Cross-compile + Release (on version tags)
```

## Tech Stack

- **Language:** Go 1.25
- **CLI framework:** `github.com/spf13/cobra`
- **K8s client:** `k8s.io/client-go` v0.32.0 (dynamic client, not typed)
- **Build:** GNU Make
- **CI/CD:** GitHub Actions
- **Container registry:** `ghcr.io/bluecontainer/kubectl-rundeck-nodes`

## Common Commands

```bash
make build              # Build binary → bin/kubectl-rundeck-nodes
make test               # Run all unit tests
make test-coverage      # Tests + HTML coverage report
make lint               # gofmt -l + go vet
make fmt                # Auto-format Go code
make tidy               # go mod tidy
make clean              # Remove build artifacts
make cross-compile      # 5-platform cross-compile
make docker-build       # Local Docker image
make integration-test   # Full e2e test (needs Docker)
make release RELEASE_VERSION=x.y.z   # Tag + push to trigger release
make release-patch                    # Auto-increment patch version
```

## Architecture & Key Patterns

### Discovery Flow
1. Query K8s API for StatefulSets + Deployments in target namespaces
2. Count total/healthy pods per workload
3. Detect Helm releases via `app.kubernetes.io/instance` label
4. Aggregate multi-workload Helm releases into single nodes
5. Optionally expand individual pods with parent references
6. Apply 5-phase filtering pipeline
7. Output as JSON (Rundeck native), YAML, or table

### 5-Phase Filtering Pipeline (filters.go)
1. **Core filtering** — workload types, health status, operator exclusion
2. **Pattern matching** — glob patterns, namespace include/exclude
3. **Deduplication** — Helm release aggregation
4. **Output customization** — tags, label-to-attribute mapping
5. **Pod discovery** — individual pod expansion

### Node Naming Convention
Format: `[cluster-prefix/]workload-type:workload-name@namespace`
Examples: `deploy:frontend@web`, `prod/sts:mydb@database`

### Custom JSON Marshaling
`RundeckNode` uses a custom `MarshalJSON` to merge dynamic attributes (labels, annotations) with core fields into a flat JSON object matching Rundeck's resource model format.

### Pre-compiled Filters
The `Filter` struct pre-compiles expensive operations (label parsing, type sets) at construction time for efficient per-node evaluation.

## Testing

- **Unit tests:** Standard Go `testing` package in `pkg/nodes/*_test.go`
- **Integration test:** Docker Compose with k3s + Rundeck, validates full plugin flow
- Always run `make test` before committing Go changes
- Always run `make lint` to check formatting

## Conventions

- Version injected via `-ldflags "-X main.version=..."` at build time
- Semantic versioning with `v` prefix for git tags (e.g., `v1.2.3`)
- **Single version source:** git tags drive the CLI binary, Rundeck plugin, and Docker image versions. The `plugin.yaml` has a dev placeholder (`0.0.0-dev`) that gets replaced at build time via `sed`. The `rundeckPluginVersion` field in `plugin.yaml` is a separate Rundeck API version and does not track the release version.
- Rundeck plugin wraps the CLI — no logic duplication between plugin and library
- Warnings (not errors) for partial discovery failures to allow partial results
- Deterministic output: sorted keys for consistent JSON/YAML across runs
- No typed K8s client — uses dynamic client via `k8s.io/client-go/dynamic`

## When Modifying

- **Adding a CLI flag:** Update `cmd/kubectl-rundeck-nodes/main.go` (Cobra flag), `pkg/nodes/types.go` (DiscoverOptions), and the relevant `pkg/nodes/*.go` implementation. Also update `rundeck-plugin/plugin.yaml` and `rundeck-plugin/contents/nodes.sh` to expose it.
- **Adding a filter:** Work in `pkg/nodes/filters.go`, add tests in `filters_test.go`.
- **Changing node attributes:** Update `pkg/nodes/attributes.go` and `types.go`.
- **Changing output format:** Update `pkg/nodes/output.go`.
