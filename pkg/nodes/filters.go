package nodes

import (
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// Filter holds pre-compiled filter state for efficient filtering.
type Filter struct {
	opts DiscoverOptions

	// Pre-computed type sets
	includeTypes map[string]bool
	excludeTypes map[string]bool

	// Pre-parsed exclude label selectors
	excludeSelectors []labels.Selector

	// Phase 2: Pre-computed namespace exclusion set
	excludeNamespaces map[string]bool

	// Phase 5: Pod filtering
	podStatusFilter map[string]bool // Pre-computed pod status filter set
}

// NewFilter creates a filter from discovery options.
func NewFilter(opts DiscoverOptions) (*Filter, error) {
	f := &Filter{
		opts:              opts,
		includeTypes:      make(map[string]bool),
		excludeTypes:      make(map[string]bool),
		excludeNamespaces: make(map[string]bool),
		podStatusFilter:   make(map[string]bool),
	}

	// Build type filter sets
	for _, t := range opts.Types {
		f.includeTypes[strings.ToLower(t)] = true
	}
	for _, t := range opts.ExcludeTypes {
		f.excludeTypes[strings.ToLower(t)] = true
	}

	// Parse exclude label selectors
	for _, sel := range opts.ExcludeLabels {
		parsed, err := labels.Parse(sel)
		if err != nil {
			return nil, err
		}
		f.excludeSelectors = append(f.excludeSelectors, parsed)
	}

	// Build namespace exclusion set
	for _, ns := range opts.ExcludeNamespaces {
		f.excludeNamespaces[ns] = true
	}

	// Build pod status filter set
	for _, status := range opts.PodStatusFilter {
		f.podStatusFilter[status] = true
	}

	return f, nil
}

// ShouldIncludeType returns true if the workload type should be included.
func (f *Filter) ShouldIncludeType(workloadType string) bool {
	t := strings.ToLower(workloadType)

	// If types specified, must be in include list
	if len(f.includeTypes) > 0 && !f.includeTypes[t] {
		return false
	}

	// Check exclude list
	if f.excludeTypes[t] {
		return false
	}

	return true
}

// ShouldExcludeByLabels returns true if the workload should be excluded based on labels.
func (f *Filter) ShouldExcludeByLabels(workloadLabels map[string]string) bool {
	if len(f.excludeSelectors) == 0 {
		return false
	}

	labelSet := labels.Set(workloadLabels)
	for _, sel := range f.excludeSelectors {
		if sel.Matches(labelSet) {
			return true
		}
	}

	return false
}

// ShouldExcludeOperator returns true if the workload matches operator patterns.
func (f *Filter) ShouldExcludeOperator(name string, workloadLabels map[string]string) bool {
	if !f.opts.ExcludeOperator {
		return false
	}

	// Check labels
	if workloadLabels["app.kubernetes.io/component"] == "operator" {
		return true
	}
	if workloadLabels["control-plane"] == "controller-manager" {
		return true
	}

	// Check name patterns
	nameLower := strings.ToLower(name)
	if matched, _ := filepath.Match("*-controller-manager", nameLower); matched {
		return true
	}
	if matched, _ := filepath.Match("*-operator", nameLower); matched {
		return true
	}

	return false
}

// ShouldExcludeByHealth returns true if the workload should be excluded based on health.
func (f *Filter) ShouldExcludeByHealth(healthyPods, totalPods int) bool {
	isHealthy := healthyPods >= totalPods

	// HealthyOnly: exclude if not all pods are healthy
	if f.opts.HealthyOnly && !isHealthy {
		return true
	}

	// UnhealthyOnly: exclude if all pods are healthy
	if f.opts.UnhealthyOnly && isHealthy {
		return true
	}

	return false
}

// ShouldExcludeByName returns true if the workload should be excluded based on name patterns.
func (f *Filter) ShouldExcludeByName(name string) bool {
	nameLower := strings.ToLower(name)

	// If include patterns specified, must match at least one
	if len(f.opts.NamePatterns) > 0 {
		matched := false
		for _, pattern := range f.opts.NamePatterns {
			if m, _ := filepath.Match(strings.ToLower(pattern), nameLower); m {
				matched = true
				break
			}
		}
		if !matched {
			return true // Exclude if no include pattern matched
		}
	}

	// Check exclude patterns
	for _, pattern := range f.opts.ExcludePatterns {
		if m, _ := filepath.Match(strings.ToLower(pattern), nameLower); m {
			return true
		}
	}

	return false
}

// ShouldExcludeByNamespace returns true if the workload should be excluded based on namespace.
func (f *Filter) ShouldExcludeByNamespace(namespace string) bool {
	// Check exact namespace exclusion
	if f.excludeNamespaces[namespace] {
		return true
	}

	// If include patterns specified, must match at least one
	if len(f.opts.NamespacePatterns) > 0 {
		matched := false
		for _, pattern := range f.opts.NamespacePatterns {
			if m, _ := filepath.Match(pattern, namespace); m {
				matched = true
				break
			}
		}
		if !matched {
			return true // Exclude if no include pattern matched
		}
	}

	// Check exclude patterns
	for _, pattern := range f.opts.ExcludeNamespacePatterns {
		if m, _ := filepath.Match(pattern, namespace); m {
			return true
		}
	}

	return false
}

// ShouldIncludeWorkload checks all filters and returns true if the workload should be included.
func (f *Filter) ShouldIncludeWorkload(obj *unstructured.Unstructured, workloadType string, healthyPods, totalPods int) bool {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	workloadLabels := obj.GetLabels()

	// Type filter
	if !f.ShouldIncludeType(workloadType) {
		return false
	}

	// Namespace filter (Phase 2)
	if f.ShouldExcludeByNamespace(namespace) {
		return false
	}

	// Name pattern filter (Phase 2)
	if f.ShouldExcludeByName(name) {
		return false
	}

	// Exclude labels
	if f.ShouldExcludeByLabels(workloadLabels) {
		return false
	}

	// Operator exclusion
	if f.ShouldExcludeOperator(name, workloadLabels) {
		return false
	}

	// Health filter
	if f.ShouldExcludeByHealth(healthyPods, totalPods) {
		return false
	}

	return true
}

// ShouldIncludeHelmRelease checks filters for a Helm release node.
func (f *Filter) ShouldIncludeHelmRelease(info *helmInfo) bool {
	// Type filter
	if !f.ShouldIncludeType(TypeHelmRelease) {
		return false
	}

	// Namespace filter (Phase 2)
	if f.ShouldExcludeByNamespace(info.namespace) {
		return false
	}

	// Name pattern filter (Phase 2) - uses release name
	if f.ShouldExcludeByName(info.release) {
		return false
	}

	// Health filter
	if f.ShouldExcludeByHealth(info.healthyPods, info.totalPods) {
		return false
	}

	// Note: We can't easily filter Helm releases by labels since they aggregate
	// multiple workloads. The workload-level filters already apply during discovery.

	return true
}

// ShouldIncludePod checks if a pod should be included based on pod-specific filters.
func (f *Filter) ShouldIncludePod(podName, phase string, isReady bool, podLabels map[string]string) bool {
	// Check pod status filter
	if len(f.podStatusFilter) > 0 && !f.podStatusFilter[phase] {
		return false
	}

	// Check pod ready filter
	if f.opts.PodReadyOnly && !isReady {
		return false
	}

	// Check pod name patterns
	if len(f.opts.PodNamePatterns) > 0 {
		matched := false
		nameLower := strings.ToLower(podName)
		for _, pattern := range f.opts.PodNamePatterns {
			if m, _ := filepath.Match(strings.ToLower(pattern), nameLower); m {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude labels (applies to pods too)
	if f.ShouldExcludeByLabels(podLabels) {
		return false
	}

	return true
}
