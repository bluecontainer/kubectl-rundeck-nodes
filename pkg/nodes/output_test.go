package nodes

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	nodes := map[string]*RundeckNode{
		"sts:myapp@default": {
			NodeName:        "sts:myapp@default",
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
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, nodes, FormatJSON)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if _, ok := result["sts:myapp@default"]; !ok {
		t.Error("Expected node key not found in output")
	}
}

func TestWriteTable(t *testing.T) {
	nodes := map[string]*RundeckNode{
		"deploy:frontend@production": {
			NodeName:        "deploy:frontend@production",
			Hostname:        "localhost",
			Tags:            "deployment,production",
			OSFamily:        "kubernetes",
			NodeExecutor:    "local",
			FileCopier:      "local",
			TargetType:      "deployment",
			TargetValue:     "frontend",
			TargetNamespace: "production",
			WorkloadKind:    "Deployment",
			WorkloadName:    "frontend",
			PodCount:        "5",
			HealthyPods:     "4",
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, nodes, FormatTable)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "frontend") {
		t.Error("Expected 'frontend' in table output")
	}
	if !strings.Contains(output, "deployment") {
		t.Error("Expected 'deployment' in table output")
	}
	if !strings.Contains(output, "4/5") {
		t.Error("Expected '4/5' pods in table output")
	}
}

func TestWriteYAML(t *testing.T) {
	nodes := map[string]*RundeckNode{
		"helm:release@ns": {
			NodeName:        "helm:release@ns",
			Hostname:        "localhost",
			Tags:            "helm-release,ns",
			OSFamily:        "kubernetes",
			NodeExecutor:    "local",
			FileCopier:      "local",
			TargetType:      "helm-release",
			TargetValue:     "release",
			TargetNamespace: "ns",
			WorkloadKind:    "StatefulSet",
			WorkloadName:    "release-db",
			PodCount:        "1",
			HealthyPods:     "1",
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, nodes, FormatYAML)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "helm-release") {
		t.Error("Expected 'helm-release' in YAML output")
	}
	// YAML now uses the same camelCase keys as JSON for consistency
	if !strings.Contains(output, "targetValue: release") {
		t.Error("Expected 'targetValue: release' in YAML output")
	}
}

func TestSortedOutput(t *testing.T) {
	nodes := map[string]*RundeckNode{
		"z-node": {NodeName: "z-node", Hostname: "localhost", OSFamily: "kubernetes", NodeExecutor: "local", FileCopier: "local"},
		"a-node": {NodeName: "a-node", Hostname: "localhost", OSFamily: "kubernetes", NodeExecutor: "local", FileCopier: "local"},
		"m-node": {NodeName: "m-node", Hostname: "localhost", OSFamily: "kubernetes", NodeExecutor: "local", FileCopier: "local"},
	}

	var buf bytes.Buffer
	err := Write(&buf, nodes, FormatJSON)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Keys should appear in sorted order in the output
	output := buf.String()
	aIdx := strings.Index(output, "a-node")
	mIdx := strings.Index(output, "m-node")
	zIdx := strings.Index(output, "z-node")

	if aIdx > mIdx || mIdx > zIdx {
		t.Error("Output is not sorted alphabetically")
	}
}

func TestClusterAttributes(t *testing.T) {
	nodes := map[string]*RundeckNode{
		"prod/sts:db@default": {
			NodeName:           "prod/sts:db@default",
			Hostname:           "localhost",
			Tags:               "statefulset,default,prod",
			OSFamily:           "kubernetes",
			NodeExecutor:       "local",
			FileCopier:         "local",
			Cluster:            "prod",
			ClusterURL:         "https://prod.k8s.example.com",
			ClusterTokenSuffix: "clusters/prod/token",
			TargetType:         "statefulset",
			TargetValue:        "db",
			TargetNamespace:    "default",
			WorkloadKind:       "StatefulSet",
			WorkloadName:       "db",
			PodCount:           "3",
			HealthyPods:        "3",
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, nodes, FormatJSON)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"cluster": "prod"`) {
		t.Error("Expected cluster attribute in output")
	}
	if !strings.Contains(output, `"clusterUrl": "https://prod.k8s.example.com"`) {
		t.Error("Expected clusterUrl attribute in output")
	}
	if !strings.Contains(output, `"clusterTokenSuffix": "clusters/prod/token"`) {
		t.Error("Expected clusterTokenSuffix attribute in output")
	}
}
