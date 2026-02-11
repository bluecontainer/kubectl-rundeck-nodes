// Package nodes provides Kubernetes workload discovery for Rundeck node sources.
// It discovers StatefulSets, Deployments, and Helm releases, outputting them
// as Rundeck resource model JSON with attributes that map to kubectl plugin
// targeting flags.
package nodes

import "encoding/json"

// RundeckNode represents a single Rundeck resource model node.
type RundeckNode struct {
	NodeName           string `json:"nodename"`
	Hostname           string `json:"hostname"`
	Tags               string `json:"tags"`
	OSFamily           string `json:"osFamily"`
	NodeExecutor       string `json:"node-executor"`
	FileCopier         string `json:"file-copier"`
	Cluster            string `json:"cluster,omitempty"`
	ClusterURL         string `json:"clusterUrl,omitempty"`
	ClusterTokenSuffix string `json:"clusterTokenSuffix,omitempty"`
	TargetType         string `json:"targetType"`
	TargetValue        string `json:"targetValue"`
	TargetNamespace    string `json:"targetNamespace"`
	WorkloadKind       string `json:"workloadKind"`
	WorkloadName       string `json:"workloadName"`
	PodCount           string `json:"podCount"`
	HealthyPods        string `json:"healthyPods"`
	Healthy            string `json:"healthy"`

	// Pod-specific fields (only set for pod nodes)
	PodInfo *PodInfo `json:"-"`

	// ExtraAttributes holds dynamic attributes from labels/annotations.
	// These are merged into the JSON output at the top level.
	ExtraAttributes map[string]string `json:"-"`
}

// ToMap converts the RundeckNode to a map for serialization.
// This ensures consistent output between JSON and YAML formats.
func (n *RundeckNode) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"nodename":        n.NodeName,
		"hostname":        n.Hostname,
		"tags":            n.Tags,
		"osFamily":        n.OSFamily,
		"node-executor":   n.NodeExecutor,
		"file-copier":     n.FileCopier,
		"targetType":      n.TargetType,
		"targetValue":     n.TargetValue,
		"targetNamespace": n.TargetNamespace,
		"workloadKind":    n.WorkloadKind,
		"workloadName":    n.WorkloadName,
		"podCount":        n.PodCount,
		"healthyPods":     n.HealthyPods,
		"healthy":         n.Healthy,
	}

	// Add optional fields
	if n.Cluster != "" {
		result["cluster"] = n.Cluster
	}
	if n.ClusterURL != "" {
		result["clusterUrl"] = n.ClusterURL
	}
	if n.ClusterTokenSuffix != "" {
		result["clusterTokenSuffix"] = n.ClusterTokenSuffix
	}

	// Add pod-specific fields if this is a pod node
	if n.PodInfo != nil {
		result["podIP"] = n.PodInfo.PodIP
		result["hostIP"] = n.PodInfo.HostIP
		result["k8sNode"] = n.PodInfo.K8sNodeName
		result["phase"] = n.PodInfo.Phase
		result["ready"] = n.PodInfo.Ready
		result["restarts"] = n.PodInfo.Restarts
		result["containerCount"] = n.PodInfo.ContainerCount
		result["readyContainers"] = n.PodInfo.ReadyContainers
		result["parentType"] = n.PodInfo.ParentType
		result["parentName"] = n.PodInfo.ParentName
		result["parentNodename"] = n.PodInfo.ParentNodename
	}

	// Merge extra attributes
	for k, v := range n.ExtraAttributes {
		result[k] = v
	}

	return result
}

// MarshalJSON implements custom JSON marshaling to merge ExtraAttributes and PodInfo.
func (n *RundeckNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.ToMap())
}

// DiscoverOptions configures workload discovery behavior.
type DiscoverOptions struct {
	// Namespace to discover workloads in. Empty string with AllNamespaces=true
	// discovers across all namespaces.
	Namespace string

	// AllNamespaces discovers workloads across all namespaces.
	AllNamespaces bool

	// LabelSelector filters workloads by label (e.g., "app=myapp").
	LabelSelector string

	// ClusterName is an identifier for multi-cluster setups.
	// If set, it's included in node names and tags.
	ClusterName string

	// ClusterURL is the Kubernetes API server URL.
	// Stored in node attributes for per-node cluster targeting.
	ClusterURL string

	// ClusterTokenSuffix is the Rundeck Key Storage path suffix for this cluster.
	// Jobs use "keys/${node.clusterTokenSuffix}" for dynamic credential selection.
	ClusterTokenSuffix string

	// DefaultTokenSuffix is the default Key Storage path suffix when
	// ClusterTokenSuffix is not set. Defaults to "rundeck/k8s-token".
	DefaultTokenSuffix string

	// Phase 1: Core Filtering Options

	// Types filters to only include these workload types.
	// Valid values: "helm-release", "statefulset", "deployment"
	// Empty means all types.
	Types []string

	// ExcludeTypes excludes these workload types from discovery.
	// Applied after Types filtering.
	ExcludeTypes []string

	// ExcludeLabels excludes workloads matching any of these label selectors.
	// Each entry is a label selector (e.g., "app=operator").
	ExcludeLabels []string

	// ExcludeOperator excludes the operator controller-manager from discovery.
	// This is a convenience flag that excludes workloads matching common operator patterns:
	// - label app.kubernetes.io/component=operator
	// - label control-plane=controller-manager
	// - name matching *-controller-manager or *-operator
	ExcludeOperator bool

	// HealthyOnly includes only workloads where all pods are running.
	HealthyOnly bool

	// UnhealthyOnly includes only workloads where some pods are not running.
	UnhealthyOnly bool

	// Phase 2: Pattern Matching Options

	// NamePatterns filters to only include workloads matching these glob patterns.
	// Uses filepath.Match syntax (e.g., "myapp-*", "*-backend").
	NamePatterns []string

	// ExcludePatterns excludes workloads matching these glob patterns.
	ExcludePatterns []string

	// ExcludeNamespaces excludes workloads in these specific namespaces.
	ExcludeNamespaces []string

	// NamespacePatterns filters to only include workloads in namespaces matching these patterns.
	NamespacePatterns []string

	// ExcludeNamespacePatterns excludes workloads in namespaces matching these patterns.
	ExcludeNamespacePatterns []string

	// Phase 4: Output Customization Options

	// AddTags adds custom static tags to all discovered nodes.
	// Example: ["env:prod", "team:platform"]
	AddTags []string

	// LabelsAsTags converts Kubernetes labels to Rundeck tags.
	// Specify label keys (e.g., "app.kubernetes.io/name", "tier").
	// The label value becomes the tag value.
	LabelsAsTags []string

	// LabelAttributes adds Kubernetes labels as node attributes.
	// Specify label keys to include as node attributes.
	// The attribute name will be "label.<key>" (dots converted to underscores).
	LabelAttributes []string

	// AnnotationAttributes adds Kubernetes annotations as node attributes.
	// Specify annotation keys to include as node attributes.
	// The attribute name will be "annotation.<key>" (dots converted to underscores).
	AnnotationAttributes []string

	// Phase 5: Pod Discovery Options

	// IncludePods expands discovered workloads to include their individual pods as nodes.
	// Each pod becomes a separate Rundeck node with parent workload reference.
	IncludePods bool

	// PodsOnly returns only pod nodes, excluding workload-level nodes.
	// Implies IncludePods=true.
	PodsOnly bool

	// PodStatusFilter filters pods by phase (e.g., "Running", "Pending").
	// Empty means all phases.
	PodStatusFilter []string

	// PodNamePatterns filters pods by name using glob patterns.
	// Example: "*-0" to match first replica of StatefulSets.
	PodNamePatterns []string

	// PodReadyOnly includes only pods where all containers are ready.
	PodReadyOnly bool

	// MaxPodsPerWorkload limits the number of pod nodes per workload.
	// 0 means no limit. Useful to avoid node explosion in large deployments.
	MaxPodsPerWorkload int
}

// helmInfo tracks Helm release information during discovery.
// Multiple workloads can belong to a single Helm release.
type helmInfo struct {
	release      string
	namespace    string
	workloadKind string
	workloadName string
	totalPods    int
	healthyPods  int
	// Labels and annotations from the first workload (for tag/attribute extraction)
	labels      map[string]string
	annotations map[string]string
}

// DefaultTokenSuffix is the default Rundeck Key Storage path suffix.
const DefaultTokenSuffix = "rundeck/k8s-token"

// Workload type constants
const (
	TypeHelmRelease = "helm-release"
	TypeStatefulSet = "statefulset"
	TypeDeployment  = "deployment"
	TypePod         = "pod"
)

// ValidTypes returns the list of valid workload types (excluding pods).
func ValidTypes() []string {
	return []string{TypeHelmRelease, TypeStatefulSet, TypeDeployment}
}

// ValidTypesWithPods returns the list of valid types including pods.
func ValidTypesWithPods() []string {
	return []string{TypeHelmRelease, TypeStatefulSet, TypeDeployment, TypePod}
}

// PodInfo holds pod-specific information for pod nodes.
type PodInfo struct {
	// PodIP is the pod's cluster IP address.
	PodIP string
	// HostIP is the IP of the node where the pod is running.
	HostIP string
	// K8sNodeName is the Kubernetes node name where the pod is scheduled.
	K8sNodeName string
	// Phase is the pod's lifecycle phase (Running, Pending, Succeeded, Failed, Unknown).
	Phase string
	// Ready indicates if all containers in the pod are ready.
	Ready bool
	// Restarts is the total restart count across all containers.
	Restarts int
	// ContainerCount is the number of containers in the pod.
	ContainerCount int
	// ReadyContainers is the number of ready containers.
	ReadyContainers int

	// Parent workload information
	ParentType     string // "statefulset", "deployment", or "helm-release"
	ParentName     string // Name of the parent workload
	ParentNodename string // Full nodename of the parent workload node
}
