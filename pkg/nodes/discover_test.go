package nodes

import (
	"testing"
)

func TestJoinTags(t *testing.T) {
	tests := []struct {
		name         string
		workloadType string
		namespace    string
		cluster      string
		expected     string
	}{
		{
			name:         "without cluster",
			workloadType: "statefulset",
			namespace:    "default",
			cluster:      "",
			expected:     "statefulset,default",
		},
		{
			name:         "with cluster",
			workloadType: "deployment",
			namespace:    "production",
			cluster:      "prod",
			expected:     "deployment,production,prod",
		},
		{
			name:         "helm release",
			workloadType: "helm-release",
			namespace:    "kube-system",
			cluster:      "staging",
			expected:     "helm-release,kube-system,staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinTags(tt.workloadType, tt.namespace, tt.cluster)
			if result != tt.expected {
				t.Errorf("joinTags(%q, %q, %q) = %q, want %q",
					tt.workloadType, tt.namespace, tt.cluster, result, tt.expected)
			}
		})
	}
}

func TestMakeNodeKey(t *testing.T) {
	tests := []struct {
		name         string
		cluster      string
		workloadType string
		workload     string
		namespace    string
		expected     string
	}{
		{
			name:         "without cluster",
			cluster:      "",
			workloadType: "sts",
			workload:     "myapp",
			namespace:    "default",
			expected:     "sts:myapp@default",
		},
		{
			name:         "with cluster",
			cluster:      "prod",
			workloadType: "deploy",
			workload:     "frontend",
			namespace:    "web",
			expected:     "prod/deploy:frontend@web",
		},
		{
			name:         "helm workload type",
			cluster:      "",
			workloadType: "helm",
			workload:     "redis",
			namespace:    "cache",
			expected:     "helm:redis@cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeNodeKey(tt.cluster, tt.workloadType, tt.workload, tt.namespace)
			if result != tt.expected {
				t.Errorf("makeNodeKey(%q, %q, %q, %q) = %q, want %q",
					tt.cluster, tt.workloadType, tt.workload, tt.namespace, result, tt.expected)
			}
		})
	}
}

func TestGetTokenSuffix(t *testing.T) {
	tests := []struct {
		name     string
		opts     DiscoverOptions
		expected string
	}{
		{
			name:     "uses cluster token suffix when set",
			opts:     DiscoverOptions{ClusterTokenSuffix: "clusters/prod/token"},
			expected: "clusters/prod/token",
		},
		{
			name:     "uses default token suffix when cluster not set",
			opts:     DiscoverOptions{DefaultTokenSuffix: "custom/token"},
			expected: "custom/token",
		},
		{
			name:     "uses built-in default when nothing set",
			opts:     DiscoverOptions{},
			expected: DefaultTokenSuffix,
		},
		{
			name: "cluster takes precedence over default",
			opts: DiscoverOptions{
				ClusterTokenSuffix: "cluster-specific",
				DefaultTokenSuffix: "default-value",
			},
			expected: "cluster-specific",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTokenSuffix(tt.opts)
			if result != tt.expected {
				t.Errorf("getTokenSuffix() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDefaultTokenSuffix(t *testing.T) {
	if DefaultTokenSuffix != "rundeck/k8s-token" {
		t.Errorf("DefaultTokenSuffix = %q, want %q", DefaultTokenSuffix, "rundeck/k8s-token")
	}
}
