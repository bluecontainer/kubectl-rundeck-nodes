package nodes

import (
	"encoding/json"
	"testing"
)

func TestBuildExtraTags(t *testing.T) {
	tests := []struct {
		name     string
		opts     DiscoverOptions
		labels   map[string]string
		expected []string
	}{
		{
			name:     "no options - empty tags",
			opts:     DiscoverOptions{},
			labels:   map[string]string{"app": "myapp"},
			expected: nil,
		},
		{
			name: "add-tags only",
			opts: DiscoverOptions{
				AddTags: []string{"env:prod", "team:platform"},
			},
			labels:   map[string]string{},
			expected: []string{"env:prod", "team:platform"},
		},
		{
			name: "labels-as-tags - label exists",
			opts: DiscoverOptions{
				LabelsAsTags: []string{"app.kubernetes.io/name", "tier"},
			},
			labels: map[string]string{
				"app.kubernetes.io/name": "myapp",
				"tier":                   "backend",
			},
			expected: []string{"app_kubernetes_io_name:myapp", "tier:backend"},
		},
		{
			name: "labels-as-tags - label missing",
			opts: DiscoverOptions{
				LabelsAsTags: []string{"app.kubernetes.io/name", "missing"},
			},
			labels: map[string]string{
				"app.kubernetes.io/name": "myapp",
			},
			expected: []string{"app_kubernetes_io_name:myapp"},
		},
		{
			name: "combined add-tags and labels-as-tags",
			opts: DiscoverOptions{
				AddTags:      []string{"env:prod"},
				LabelsAsTags: []string{"tier"},
			},
			labels: map[string]string{
				"tier": "frontend",
			},
			expected: []string{"env:prod", "tier:frontend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExtraTags(tt.opts, tt.labels)
			if len(got) != len(tt.expected) {
				t.Errorf("buildExtraTags() = %v, want %v", got, tt.expected)
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("buildExtraTags()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestBuildExtraAttributes(t *testing.T) {
	tests := []struct {
		name        string
		opts        DiscoverOptions
		labels      map[string]string
		annotations map[string]string
		expected    map[string]string
	}{
		{
			name:        "no options - empty attributes",
			opts:        DiscoverOptions{},
			labels:      map[string]string{"app": "myapp"},
			annotations: map[string]string{"note": "test"},
			expected:    map[string]string{},
		},
		{
			name: "label-attributes",
			opts: DiscoverOptions{
				LabelAttributes: []string{"app.kubernetes.io/version", "tier"},
			},
			labels: map[string]string{
				"app.kubernetes.io/version": "v1.2.3",
				"tier":                      "backend",
			},
			annotations: map[string]string{},
			expected: map[string]string{
				"label_app_kubernetes_io_version": "v1.2.3",
				"label_tier":                      "backend",
			},
		},
		{
			name: "annotation-attributes",
			opts: DiscoverOptions{
				AnnotationAttributes: []string{"prometheus.io/scrape", "custom/note"},
			},
			labels: map[string]string{},
			annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"custom/note":          "important",
			},
			expected: map[string]string{
				"annotation_prometheus_io_scrape": "true",
				"annotation_custom_note":          "important",
			},
		},
		{
			name: "combined labels and annotations",
			opts: DiscoverOptions{
				LabelAttributes:      []string{"version"},
				AnnotationAttributes: []string{"description"},
			},
			labels: map[string]string{
				"version": "v2.0",
			},
			annotations: map[string]string{
				"description": "My service",
			},
			expected: map[string]string{
				"label_version":          "v2.0",
				"annotation_description": "My service",
			},
		},
		{
			name: "missing label and annotation",
			opts: DiscoverOptions{
				LabelAttributes:      []string{"version", "missing"},
				AnnotationAttributes: []string{"note", "alsomissing"},
			},
			labels: map[string]string{
				"version": "v1",
			},
			annotations: map[string]string{
				"note": "test",
			},
			expected: map[string]string{
				"label_version":   "v1",
				"annotation_note": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExtraAttributes(tt.opts, tt.labels, tt.annotations)
			if len(got) != len(tt.expected) {
				t.Errorf("buildExtraAttributes() = %v, want %v", got, tt.expected)
				return
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("buildExtraAttributes()[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestSanitizeTagKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"app", "app"},
		{"app.kubernetes.io/name", "app_kubernetes_io_name"},
		{"my-label", "my_label"},
		{"complex.label/with-dashes", "complex_label_with_dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeTagKey(tt.input); got != tt.expected {
				t.Errorf("sanitizeTagKey(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMergeTagStrings(t *testing.T) {
	tests := []struct {
		name      string
		baseTags  string
		extraTags []string
		expected  string
	}{
		{
			name:      "empty extra tags",
			baseTags:  "statefulset,default",
			extraTags: nil,
			expected:  "statefulset,default",
		},
		{
			name:      "merge with extra tags",
			baseTags:  "statefulset,default",
			extraTags: []string{"env:prod", "tier:backend"},
			expected:  "statefulset,default,env:prod,tier:backend",
		},
		{
			name:      "empty base tags",
			baseTags:  "",
			extraTags: []string{"env:prod"},
			expected:  "env:prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeTagStrings(tt.baseTags, tt.extraTags); got != tt.expected {
				t.Errorf("mergeTagStrings() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRundeckNodeMarshalJSON(t *testing.T) {
	node := &RundeckNode{
		NodeName:        "test-node",
		Hostname:        "localhost",
		Tags:            "statefulset,default",
		OSFamily:        "kubernetes",
		NodeExecutor:    "local",
		FileCopier:      "local",
		TargetType:      "statefulset",
		TargetValue:     "myapp",
		TargetNamespace: "default",
		WorkloadKind:    "StatefulSet",
		WorkloadName:    "myapp",
		PodCount:        "3",
		HealthyPods:     "3",
		Healthy:         "true",
		ExtraAttributes: map[string]string{
			"label_version":       "v1.0",
			"annotation_oncall":   "team-a",
		},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Unmarshal to verify the extra attributes are at top level
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Check standard fields
	if result["nodename"] != "test-node" {
		t.Errorf("nodename = %v, want test-node", result["nodename"])
	}
	if result["targetType"] != "statefulset" {
		t.Errorf("targetType = %v, want statefulset", result["targetType"])
	}

	// Check extra attributes are at top level
	if result["label_version"] != "v1.0" {
		t.Errorf("label_version = %v, want v1.0", result["label_version"])
	}
	if result["annotation_oncall"] != "team-a" {
		t.Errorf("annotation_oncall = %v, want team-a", result["annotation_oncall"])
	}

	// Verify optional cluster fields are omitted when empty
	if _, exists := result["cluster"]; exists {
		t.Errorf("cluster should be omitted when empty")
	}
}

func TestRundeckNodeMarshalJSONWithCluster(t *testing.T) {
	node := &RundeckNode{
		NodeName:           "test-node",
		Hostname:           "localhost",
		Tags:               "statefulset,default",
		OSFamily:           "kubernetes",
		NodeExecutor:       "local",
		FileCopier:         "local",
		Cluster:            "prod",
		ClusterURL:         "https://k8s.example.com",
		ClusterTokenSuffix: "clusters/prod/token",
		TargetType:         "statefulset",
		TargetValue:        "myapp",
		TargetNamespace:    "default",
		WorkloadKind:       "StatefulSet",
		WorkloadName:       "myapp",
		PodCount:           "3",
		HealthyPods:        "3",
		Healthy:            "true",
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Check cluster fields are included
	if result["cluster"] != "prod" {
		t.Errorf("cluster = %v, want prod", result["cluster"])
	}
	if result["clusterUrl"] != "https://k8s.example.com" {
		t.Errorf("clusterUrl = %v, want https://k8s.example.com", result["clusterUrl"])
	}
	if result["clusterTokenSuffix"] != "clusters/prod/token" {
		t.Errorf("clusterTokenSuffix = %v, want clusters/prod/token", result["clusterTokenSuffix"])
	}
}
