package nodes

import (
	"testing"
)

func TestNewFilter(t *testing.T) {
	tests := []struct {
		name    string
		opts    DiscoverOptions
		wantErr bool
	}{
		{
			name: "empty options",
			opts: DiscoverOptions{},
		},
		{
			name: "with types",
			opts: DiscoverOptions{
				Types: []string{"statefulset", "deployment"},
			},
		},
		{
			name: "with exclude labels",
			opts: DiscoverOptions{
				ExcludeLabels: []string{"app=operator", "tier=control-plane"},
			},
		},
		{
			name: "invalid label selector",
			opts: DiscoverOptions{
				ExcludeLabels: []string{"invalid===selector"},
			},
			wantErr: true,
		},
		{
			name: "with phase 2 options",
			opts: DiscoverOptions{
				NamePatterns:             []string{"myapp-*"},
				ExcludePatterns:          []string{"*-test"},
				ExcludeNamespaces:        []string{"kube-system"},
				NamespacePatterns:        []string{"prod-*"},
				ExcludeNamespacePatterns: []string{"*-temp"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFilter(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilter_ShouldIncludeType(t *testing.T) {
	tests := []struct {
		name         string
		types        []string
		excludeTypes []string
		workloadType string
		want         bool
	}{
		{
			name:         "empty filters - include all",
			workloadType: "statefulset",
			want:         true,
		},
		{
			name:         "types filter - included",
			types:        []string{"statefulset", "deployment"},
			workloadType: "statefulset",
			want:         true,
		},
		{
			name:         "types filter - excluded",
			types:        []string{"statefulset"},
			workloadType: "deployment",
			want:         false,
		},
		{
			name:         "exclude-types filter",
			excludeTypes: []string{"deployment"},
			workloadType: "deployment",
			want:         false,
		},
		{
			name:         "exclude-types filter - not excluded",
			excludeTypes: []string{"deployment"},
			workloadType: "statefulset",
			want:         true,
		},
		{
			name:         "types and exclude-types - exclude takes precedence",
			types:        []string{"statefulset", "deployment"},
			excludeTypes: []string{"deployment"},
			workloadType: "deployment",
			want:         false,
		},
		{
			name:         "helm-release type",
			types:        []string{"helm-release"},
			workloadType: "helm-release",
			want:         true,
		},
		{
			name:         "case insensitive",
			types:        []string{"StatefulSet"},
			workloadType: "statefulset",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				Types:        tt.types,
				ExcludeTypes: tt.excludeTypes,
			})
			if got := f.ShouldIncludeType(tt.workloadType); got != tt.want {
				t.Errorf("ShouldIncludeType(%q) = %v, want %v", tt.workloadType, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldExcludeByLabels(t *testing.T) {
	tests := []struct {
		name           string
		excludeLabels  []string
		workloadLabels map[string]string
		want           bool
	}{
		{
			name:           "no exclude labels",
			workloadLabels: map[string]string{"app": "myapp"},
			want:           false,
		},
		{
			name:           "matching exclude label",
			excludeLabels:  []string{"app=operator"},
			workloadLabels: map[string]string{"app": "operator"},
			want:           true,
		},
		{
			name:           "non-matching exclude label",
			excludeLabels:  []string{"app=operator"},
			workloadLabels: map[string]string{"app": "myapp"},
			want:           false,
		},
		{
			name:           "multiple exclude labels - one matches",
			excludeLabels:  []string{"app=operator", "tier=control-plane"},
			workloadLabels: map[string]string{"tier": "control-plane"},
			want:           true,
		},
		{
			name:           "set-based selector",
			excludeLabels:  []string{"environment in (prod,staging)"},
			workloadLabels: map[string]string{"environment": "prod"},
			want:           true,
		},
		{
			name:           "set-based selector - no match",
			excludeLabels:  []string{"environment in (prod,staging)"},
			workloadLabels: map[string]string{"environment": "dev"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFilter(DiscoverOptions{
				ExcludeLabels: tt.excludeLabels,
			})
			if err != nil {
				t.Fatalf("NewFilter() error = %v", err)
			}
			if got := f.ShouldExcludeByLabels(tt.workloadLabels); got != tt.want {
				t.Errorf("ShouldExcludeByLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldExcludeOperator(t *testing.T) {
	tests := []struct {
		name            string
		excludeOperator bool
		workloadName    string
		workloadLabels  map[string]string
		want            bool
	}{
		{
			name:            "exclude disabled",
			excludeOperator: false,
			workloadName:    "myapp-controller-manager",
			want:            false,
		},
		{
			name:            "exclude by component label",
			excludeOperator: true,
			workloadName:    "myapp",
			workloadLabels:  map[string]string{"app.kubernetes.io/component": "operator"},
			want:            true,
		},
		{
			name:            "exclude by control-plane label",
			excludeOperator: true,
			workloadName:    "myapp",
			workloadLabels:  map[string]string{"control-plane": "controller-manager"},
			want:            true,
		},
		{
			name:            "exclude by name pattern - controller-manager",
			excludeOperator: true,
			workloadName:    "petstore-controller-manager",
			workloadLabels:  map[string]string{},
			want:            true,
		},
		{
			name:            "exclude by name pattern - operator",
			excludeOperator: true,
			workloadName:    "my-operator",
			workloadLabels:  map[string]string{},
			want:            true,
		},
		{
			name:            "not excluded - normal workload",
			excludeOperator: true,
			workloadName:    "myapp-backend",
			workloadLabels:  map[string]string{"app": "myapp"},
			want:            false,
		},
		{
			name:            "not excluded - name contains operator but doesn't end with it",
			excludeOperator: true,
			workloadName:    "operator-metrics",
			workloadLabels:  map[string]string{},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				ExcludeOperator: tt.excludeOperator,
			})
			if got := f.ShouldExcludeOperator(tt.workloadName, tt.workloadLabels); got != tt.want {
				t.Errorf("ShouldExcludeOperator(%q, %v) = %v, want %v",
					tt.workloadName, tt.workloadLabels, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldExcludeByHealth(t *testing.T) {
	tests := []struct {
		name          string
		healthyOnly   bool
		unhealthyOnly bool
		healthyPods   int
		totalPods     int
		want          bool
	}{
		{
			name:        "no filter - include all",
			healthyOnly: false,
			healthyPods: 1,
			totalPods:   3,
			want:        false,
		},
		{
			name:        "healthy-only - all healthy",
			healthyOnly: true,
			healthyPods: 3,
			totalPods:   3,
			want:        false,
		},
		{
			name:        "healthy-only - some unhealthy",
			healthyOnly: true,
			healthyPods: 2,
			totalPods:   3,
			want:        true,
		},
		{
			name:        "healthy-only - none healthy",
			healthyOnly: true,
			healthyPods: 0,
			totalPods:   3,
			want:        true,
		},
		{
			name:        "healthy-only - zero replicas",
			healthyOnly: true,
			healthyPods: 0,
			totalPods:   0,
			want:        false,
		},
		{
			name:          "unhealthy-only - all healthy (exclude)",
			unhealthyOnly: true,
			healthyPods:   3,
			totalPods:     3,
			want:          true,
		},
		{
			name:          "unhealthy-only - some unhealthy (include)",
			unhealthyOnly: true,
			healthyPods:   2,
			totalPods:     3,
			want:          false,
		},
		{
			name:          "unhealthy-only - none healthy (include)",
			unhealthyOnly: true,
			healthyPods:   0,
			totalPods:     3,
			want:          false,
		},
		{
			name:          "unhealthy-only - zero replicas (exclude as healthy)",
			unhealthyOnly: true,
			healthyPods:   0,
			totalPods:     0,
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				HealthyOnly:   tt.healthyOnly,
				UnhealthyOnly: tt.unhealthyOnly,
			})
			if got := f.ShouldExcludeByHealth(tt.healthyPods, tt.totalPods); got != tt.want {
				t.Errorf("ShouldExcludeByHealth(%d, %d) = %v, want %v",
					tt.healthyPods, tt.totalPods, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldIncludeHelmRelease(t *testing.T) {
	tests := []struct {
		name        string
		types       []string
		healthyOnly bool
		info        *helmInfo
		want        bool
	}{
		{
			name: "no filters",
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:  "helm-release type allowed",
			types: []string{"helm-release"},
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:  "helm-release type not allowed",
			types: []string{"statefulset"},
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: false,
		},
		{
			name:        "healthy-only - healthy",
			healthyOnly: true,
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:        "healthy-only - unhealthy",
			healthyOnly: true,
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 2,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				Types:       tt.types,
				HealthyOnly: tt.healthyOnly,
			})
			if got := f.ShouldIncludeHelmRelease(tt.info); got != tt.want {
				t.Errorf("ShouldIncludeHelmRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Phase 2: Name pattern tests

func TestFilter_ShouldExcludeByName(t *testing.T) {
	tests := []struct {
		name            string
		namePatterns    []string
		excludePatterns []string
		workloadName    string
		want            bool
	}{
		{
			name:         "no patterns - include all",
			workloadName: "myapp-backend",
			want:         false,
		},
		{
			name:         "name-pattern matches",
			namePatterns: []string{"myapp-*"},
			workloadName: "myapp-backend",
			want:         false,
		},
		{
			name:         "name-pattern does not match",
			namePatterns: []string{"myapp-*"},
			workloadName: "other-service",
			want:         true, // excluded because no include pattern matched
		},
		{
			name:         "multiple name-patterns - one matches",
			namePatterns: []string{"myapp-*", "*-backend"},
			workloadName: "other-backend",
			want:         false,
		},
		{
			name:         "multiple name-patterns - none match",
			namePatterns: []string{"myapp-*", "other-*"},
			workloadName: "service-frontend",
			want:         true, // excluded
		},
		{
			name:            "exclude-pattern matches",
			excludePatterns: []string{"*-test"},
			workloadName:    "myapp-test",
			want:            true, // excluded
		},
		{
			name:            "exclude-pattern does not match",
			excludePatterns: []string{"*-test"},
			workloadName:    "myapp-backend",
			want:            false,
		},
		{
			name:            "include and exclude - include matches, exclude does not",
			namePatterns:    []string{"myapp-*"},
			excludePatterns: []string{"*-test"},
			workloadName:    "myapp-backend",
			want:            false,
		},
		{
			name:            "include and exclude - both match (exclude wins)",
			namePatterns:    []string{"myapp-*"},
			excludePatterns: []string{"*-test"},
			workloadName:    "myapp-test",
			want:            true, // excluded
		},
		{
			name:         "case insensitive matching",
			namePatterns: []string{"MyApp-*"},
			workloadName: "myapp-backend",
			want:         false,
		},
		{
			name:         "exact name pattern",
			namePatterns: []string{"myapp"},
			workloadName: "myapp",
			want:         false,
		},
		{
			name:         "exact name pattern - no match",
			namePatterns: []string{"myapp"},
			workloadName: "myapp-backend",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				NamePatterns:    tt.namePatterns,
				ExcludePatterns: tt.excludePatterns,
			})
			if got := f.ShouldExcludeByName(tt.workloadName); got != tt.want {
				t.Errorf("ShouldExcludeByName(%q) = %v, want %v", tt.workloadName, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldExcludeByNamespace(t *testing.T) {
	tests := []struct {
		name                     string
		excludeNamespaces        []string
		namespacePatterns        []string
		excludeNamespacePatterns []string
		namespace                string
		want                     bool
	}{
		{
			name:      "no filters - include all",
			namespace: "default",
			want:      false,
		},
		{
			name:              "exclude-namespaces matches",
			excludeNamespaces: []string{"kube-system", "kube-public"},
			namespace:         "kube-system",
			want:              true,
		},
		{
			name:              "exclude-namespaces does not match",
			excludeNamespaces: []string{"kube-system", "kube-public"},
			namespace:         "default",
			want:              false,
		},
		{
			name:              "namespace-pattern matches",
			namespacePatterns: []string{"prod-*"},
			namespace:         "prod-us-east",
			want:              false,
		},
		{
			name:              "namespace-pattern does not match",
			namespacePatterns: []string{"prod-*"},
			namespace:         "staging-us-east",
			want:              true, // excluded
		},
		{
			name:              "multiple namespace-patterns - one matches",
			namespacePatterns: []string{"prod-*", "staging-*"},
			namespace:         "staging-us-west",
			want:              false,
		},
		{
			name:                     "exclude-namespace-pattern matches",
			excludeNamespacePatterns: []string{"*-test"},
			namespace:                "feature-test",
			want:                     true, // excluded
		},
		{
			name:                     "exclude-namespace-pattern does not match",
			excludeNamespacePatterns: []string{"*-test"},
			namespace:                "production",
			want:                     false,
		},
		{
			name:              "include pattern and exclude namespace - exclude wins",
			namespacePatterns: []string{"*"},
			excludeNamespaces: []string{"kube-system"},
			namespace:         "kube-system",
			want:              true, // excluded
		},
		{
			name:                     "include and exclude patterns - both match (exclude wins)",
			namespacePatterns:        []string{"*-west"},
			excludeNamespacePatterns: []string{"staging-*"},
			namespace:                "staging-west",
			want:                     true, // excluded
		},
		{
			name:              "kube-* pattern",
			namespacePatterns: []string{"kube-*"},
			namespace:         "kube-system",
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				ExcludeNamespaces:        tt.excludeNamespaces,
				NamespacePatterns:        tt.namespacePatterns,
				ExcludeNamespacePatterns: tt.excludeNamespacePatterns,
			})
			if got := f.ShouldExcludeByNamespace(tt.namespace); got != tt.want {
				t.Errorf("ShouldExcludeByNamespace(%q) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldIncludeHelmRelease_Phase2(t *testing.T) {
	tests := []struct {
		name              string
		namePatterns      []string
		excludePatterns   []string
		excludeNamespaces []string
		namespacePatterns []string
		info              *helmInfo
		want              bool
	}{
		{
			name: "no filters",
			info: &helmInfo{
				release:     "myrelease",
				namespace:   "default",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:         "name-pattern matches release name",
			namePatterns: []string{"my*"},
			info: &helmInfo{
				release:     "myrelease",
				namespace:   "default",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:         "name-pattern does not match release name",
			namePatterns: []string{"other*"},
			info: &helmInfo{
				release:     "myrelease",
				namespace:   "default",
				totalPods:   3,
				healthyPods: 3,
			},
			want: false,
		},
		{
			name:              "exclude-namespaces matches",
			excludeNamespaces: []string{"kube-system"},
			info: &helmInfo{
				release:     "myrelease",
				namespace:   "kube-system",
				totalPods:   3,
				healthyPods: 3,
			},
			want: false,
		},
		{
			name:              "namespace-pattern matches",
			namespacePatterns: []string{"prod-*"},
			info: &helmInfo{
				release:     "myrelease",
				namespace:   "prod-us-east",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:              "namespace-pattern does not match",
			namespacePatterns: []string{"prod-*"},
			info: &helmInfo{
				release:     "myrelease",
				namespace:   "staging-us-east",
				totalPods:   3,
				healthyPods: 3,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				NamePatterns:      tt.namePatterns,
				ExcludePatterns:   tt.excludePatterns,
				ExcludeNamespaces: tt.excludeNamespaces,
				NamespacePatterns: tt.namespacePatterns,
			})
			if got := f.ShouldIncludeHelmRelease(tt.info); got != tt.want {
				t.Errorf("ShouldIncludeHelmRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidTypes(t *testing.T) {
	types := ValidTypes()
	if len(types) != 3 {
		t.Errorf("ValidTypes() returned %d types, want 3", len(types))
	}

	expected := map[string]bool{
		TypeHelmRelease: true,
		TypeStatefulSet: true,
		TypeDeployment:  true,
	}

	for _, typ := range types {
		if !expected[typ] {
			t.Errorf("Unexpected type in ValidTypes(): %s", typ)
		}
	}
}

func TestValidTypesWithPods(t *testing.T) {
	types := ValidTypesWithPods()
	if len(types) != 4 {
		t.Errorf("ValidTypesWithPods() returned %d types, want 4", len(types))
	}

	expected := map[string]bool{
		TypeHelmRelease: true,
		TypeStatefulSet: true,
		TypeDeployment:  true,
		TypePod:         true,
	}

	for _, typ := range types {
		if !expected[typ] {
			t.Errorf("Unexpected type in ValidTypesWithPods(): %s", typ)
		}
	}
}

// Phase 5: Pod filtering tests

func TestFilter_ShouldIncludePod(t *testing.T) {
	tests := []struct {
		name            string
		podStatusFilter []string
		podNamePatterns []string
		podReadyOnly    bool
		excludeLabels   []string
		podName         string
		phase           string
		isReady         bool
		podLabels       map[string]string
		want            bool
	}{
		{
			name:    "no filters - include all",
			podName: "myapp-0",
			phase:   "Running",
			isReady: true,
			want:    true,
		},
		{
			name:            "pod-status filter - matches",
			podStatusFilter: []string{"Running"},
			podName:         "myapp-0",
			phase:           "Running",
			isReady:         true,
			want:            true,
		},
		{
			name:            "pod-status filter - no match",
			podStatusFilter: []string{"Running"},
			podName:         "myapp-0",
			phase:           "Pending",
			isReady:         false,
			want:            false,
		},
		{
			name:            "pod-status filter - multiple statuses",
			podStatusFilter: []string{"Running", "Pending"},
			podName:         "myapp-0",
			phase:           "Pending",
			isReady:         false,
			want:            true,
		},
		{
			name:         "pod-ready-only - ready",
			podReadyOnly: true,
			podName:      "myapp-0",
			phase:        "Running",
			isReady:      true,
			want:         true,
		},
		{
			name:         "pod-ready-only - not ready",
			podReadyOnly: true,
			podName:      "myapp-0",
			phase:        "Running",
			isReady:      false,
			want:         false,
		},
		{
			name:            "pod-name-pattern matches",
			podNamePatterns: []string{"*-0"},
			podName:         "myapp-0",
			phase:           "Running",
			isReady:         true,
			want:            true,
		},
		{
			name:            "pod-name-pattern does not match",
			podNamePatterns: []string{"*-0"},
			podName:         "myapp-1",
			phase:           "Running",
			isReady:         true,
			want:            false,
		},
		{
			name:            "multiple pod-name-patterns - one matches",
			podNamePatterns: []string{"*-0", "*-1"},
			podName:         "myapp-1",
			phase:           "Running",
			isReady:         true,
			want:            true,
		},
		{
			name:            "pod-name-pattern case insensitive",
			podNamePatterns: []string{"MyApp-*"},
			podName:         "myapp-0",
			phase:           "Running",
			isReady:         true,
			want:            true,
		},
		{
			name:          "exclude-labels applies to pods",
			excludeLabels: []string{"app=operator"},
			podName:       "operator-0",
			phase:         "Running",
			isReady:       true,
			podLabels:     map[string]string{"app": "operator"},
			want:          false,
		},
		{
			name:          "exclude-labels - no match",
			excludeLabels: []string{"app=operator"},
			podName:       "myapp-0",
			phase:         "Running",
			isReady:       true,
			podLabels:     map[string]string{"app": "myapp"},
			want:          true,
		},
		{
			name:            "combined filters - all pass",
			podStatusFilter: []string{"Running"},
			podNamePatterns: []string{"myapp-*"},
			podReadyOnly:    true,
			podName:         "myapp-0",
			phase:           "Running",
			isReady:         true,
			want:            true,
		},
		{
			name:            "combined filters - status fails",
			podStatusFilter: []string{"Running"},
			podNamePatterns: []string{"myapp-*"},
			podReadyOnly:    true,
			podName:         "myapp-0",
			phase:           "Pending",
			isReady:         false,
			want:            false,
		},
		{
			name:            "combined filters - name pattern fails",
			podStatusFilter: []string{"Running"},
			podNamePatterns: []string{"other-*"},
			podReadyOnly:    true,
			podName:         "myapp-0",
			phase:           "Running",
			isReady:         true,
			want:            false,
		},
		{
			name:            "combined filters - ready fails",
			podStatusFilter: []string{"Running"},
			podNamePatterns: []string{"myapp-*"},
			podReadyOnly:    true,
			podName:         "myapp-0",
			phase:           "Running",
			isReady:         false,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFilter(DiscoverOptions{
				PodStatusFilter: tt.podStatusFilter,
				PodNamePatterns: tt.podNamePatterns,
				PodReadyOnly:    tt.podReadyOnly,
				ExcludeLabels:   tt.excludeLabels,
			})
			if err != nil {
				t.Fatalf("NewFilter() error = %v", err)
			}

			podLabels := tt.podLabels
			if podLabels == nil {
				podLabels = map[string]string{}
			}

			if got := f.ShouldIncludePod(tt.podName, tt.phase, tt.isReady, podLabels); got != tt.want {
				t.Errorf("ShouldIncludePod(%q, %q, %v, %v) = %v, want %v",
					tt.podName, tt.phase, tt.isReady, podLabels, got, tt.want)
			}
		})
	}
}

func TestNewFilter_WithPodOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    DiscoverOptions
		wantErr bool
	}{
		{
			name: "with pod status filter",
			opts: DiscoverOptions{
				PodStatusFilter: []string{"Running", "Pending"},
			},
		},
		{
			name: "with pod name patterns",
			opts: DiscoverOptions{
				PodNamePatterns: []string{"*-0", "*-1"},
			},
		},
		{
			name: "with pod ready only",
			opts: DiscoverOptions{
				PodReadyOnly: true,
			},
		},
		{
			name: "with all pod options",
			opts: DiscoverOptions{
				IncludePods:        true,
				PodsOnly:           false,
				PodStatusFilter:    []string{"Running"},
				PodNamePatterns:    []string{"*-0"},
				PodReadyOnly:       true,
				MaxPodsPerWorkload: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFilter(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
