package nodes

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	stsGVR    = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	deployGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	podGVR    = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
)

// Discover queries the Kubernetes API for workloads and returns Rundeck nodes.
// It discovers StatefulSets and Deployments, with Helm release detection and
// deduplication based on the app.kubernetes.io/instance label.
// When IncludePods or PodsOnly is set, it also discovers individual pods.
func Discover(ctx context.Context, client dynamic.Interface, opts DiscoverOptions) (map[string]*RundeckNode, error) {
	// PodsOnly implies IncludePods
	if opts.PodsOnly {
		opts.IncludePods = true
	}

	// Create filter from options
	filter, err := NewFilter(opts)
	if err != nil {
		return nil, fmt.Errorf("invalid filter options: %w", err)
	}

	listOpts := metav1.ListOptions{}
	if opts.LabelSelector != "" {
		listOpts.LabelSelector = opts.LabelSelector
	}

	nodes := make(map[string]*RundeckNode)
	helmReleases := make(map[string]*helmInfo)          // key: "release@namespace"
	workloadPods := make([]workloadPodInfo, 0)          // Track workloads for pod expansion
	helmWorkloads := make(map[string][]workloadPodInfo) // Track workloads per helm release

	// Discover StatefulSets (if type is allowed)
	if filter.ShouldIncludeType(TypeStatefulSet) {
		podInfos, err := discoverStatefulSets(ctx, client, opts, listOpts, filter, nodes, helmReleases)
		if err != nil {
			// Log but don't fail - allow partial discovery
			fmt.Printf("Warning: failed to list StatefulSets: %v\n", err)
		}
		// Track for pod expansion
		for _, pi := range podInfos {
			if pi.workloadType == TypeHelmRelease {
				// Group by helm release key
				key := pi.workloadName + "@" + pi.namespace
				helmWorkloads[key] = append(helmWorkloads[key], pi)
			} else {
				workloadPods = append(workloadPods, pi)
			}
		}
	}

	// Discover Deployments (if type is allowed)
	if filter.ShouldIncludeType(TypeDeployment) {
		podInfos, err := discoverDeployments(ctx, client, opts, listOpts, filter, nodes, helmReleases)
		if err != nil {
			fmt.Printf("Warning: failed to list Deployments: %v\n", err)
		}
		// Track for pod expansion
		for _, pi := range podInfos {
			if pi.workloadType == TypeHelmRelease {
				// Group by helm release key
				key := pi.workloadName + "@" + pi.namespace
				helmWorkloads[key] = append(helmWorkloads[key], pi)
			} else {
				workloadPods = append(workloadPods, pi)
			}
		}
	}

	// Add Helm release nodes (if type is allowed)
	for _, info := range helmReleases {
		if !filter.ShouldIncludeHelmRelease(info) {
			continue
		}

		// Build extra tags and attributes from stored labels/annotations
		baseTags := joinTags("helm-release", info.namespace, opts.ClusterName)
		extraTags := buildExtraTags(opts, info.labels)
		allTags := mergeTagStrings(baseTags, extraTags)
		extraAttrs := buildExtraAttributes(opts, info.labels, info.annotations)

		nodeKey := makeNodeKey(opts.ClusterName, "helm", info.release, info.namespace)

		// Only add workload node if not PodsOnly mode
		if !opts.PodsOnly {
			nodes[nodeKey] = &RundeckNode{
				NodeName:           nodeKey,
				Hostname:           "localhost",
				Tags:               allTags,
				OSFamily:           "kubernetes",
				NodeExecutor:       "local",
				FileCopier:         "local",
				Cluster:            opts.ClusterName,
				ClusterURL:         opts.ClusterURL,
				ClusterTokenSuffix: getTokenSuffix(opts),
				TargetType:         "helm-release",
				TargetValue:        info.release,
				TargetNamespace:    info.namespace,
				WorkloadKind:       info.workloadKind,
				WorkloadName:       info.workloadName,
				PodCount:           fmt.Sprintf("%d", info.totalPods),
				HealthyPods:        fmt.Sprintf("%d", info.healthyPods),
				Healthy:            fmt.Sprintf("%t", info.healthyPods >= info.totalPods),
				ExtraAttributes:    extraAttrs,
			}
		}

		// Track helm release workloads for pod expansion
		if opts.IncludePods {
			key := info.release + "@" + info.namespace
			for _, wpi := range helmWorkloads[key] {
				// Update parent info to point to helm release
				wpi.workloadType = TypeHelmRelease
				wpi.workloadName = info.release
				wpi.workloadKey = nodeKey
				workloadPods = append(workloadPods, wpi)
			}
		}
	}

	// Discover pods for workloads if enabled
	if opts.IncludePods {
		for _, wpi := range workloadPods {
			podNodes, err := discoverPodsForWorkload(ctx, client, opts, filter, wpi)
			if err != nil {
				fmt.Printf("Warning: failed to list pods for %s/%s: %v\n", wpi.workloadType, wpi.workloadName, err)
				continue
			}
			for _, podNode := range podNodes {
				nodes[podNode.NodeName] = podNode
			}
		}
	}

	// If PodsOnly, remove workload nodes (keep only pods)
	if opts.PodsOnly {
		for key, node := range nodes {
			if node.TargetType != TypePod {
				delete(nodes, key)
			}
		}
	}

	return nodes, nil
}

func discoverStatefulSets(ctx context.Context, client dynamic.Interface, opts DiscoverOptions, listOpts metav1.ListOptions, filter *Filter, nodes map[string]*RundeckNode, helmReleases map[string]*helmInfo) ([]workloadPodInfo, error) {
	var stsList *unstructured.UnstructuredList
	var err error
	var podInfos []workloadPodInfo

	if opts.AllNamespaces {
		stsList, err = client.Resource(stsGVR).Namespace("").List(ctx, listOpts)
	} else {
		stsList, err = client.Resource(stsGVR).Namespace(opts.Namespace).List(ctx, listOpts)
	}
	if err != nil {
		return nil, err
	}

	for _, sts := range stsList.Items {
		stsName := sts.GetName()
		stsNS := sts.GetNamespace()
		labels := sts.GetLabels()
		annotations := sts.GetAnnotations()
		selectorLabels := getSelectorLabels(&sts)
		replicas, _, _ := unstructured.NestedInt64(sts.Object, "spec", "replicas")
		if replicas == 0 {
			replicas = 1
		}

		healthy := countHealthyPods(ctx, client, stsNS, selectorLabels)

		// Apply filters (exclude labels, operator exclusion)
		if filter.ShouldExcludeByLabels(labels) {
			continue
		}
		if filter.ShouldExcludeOperator(stsName, labels) {
			continue
		}

		helmRelease := labels["app.kubernetes.io/instance"]
		if helmRelease != "" {
			key := helmRelease + "@" + stsNS
			if existing, ok := helmReleases[key]; ok {
				existing.totalPods += int(replicas)
				existing.healthyPods += healthy
				if int(replicas) > 0 && existing.workloadKind == "" {
					existing.workloadKind = "StatefulSet"
					existing.workloadName = stsName
				}
			} else {
				helmReleases[key] = &helmInfo{
					release:      helmRelease,
					namespace:    stsNS,
					workloadKind: "StatefulSet",
					workloadName: stsName,
					totalPods:    int(replicas),
					healthyPods:  healthy,
					labels:       labels,
					annotations:  annotations,
				}
			}
			// Track for pod expansion (helm release)
			if opts.IncludePods {
				podInfos = append(podInfos, workloadPodInfo{
					workloadType:   TypeHelmRelease,
					workloadName:   helmRelease,
					workloadKey:    "", // Will be set later when helm node is created
					namespace:      stsNS,
					selectorLabels: selectorLabels,
					labels:         labels,
					annotations:    annotations,
				})
			}
		} else {
			// Apply health filter for non-Helm workloads
			if filter.ShouldExcludeByHealth(healthy, int(replicas)) {
				continue
			}

			// Build extra tags and attributes
			baseTags := joinTags("statefulset", stsNS, opts.ClusterName)
			extraTags := buildExtraTags(opts, labels)
			allTags := mergeTagStrings(baseTags, extraTags)
			extraAttrs := buildExtraAttributes(opts, labels, annotations)

			nodeKey := makeNodeKey(opts.ClusterName, "sts", stsName, stsNS)

			// Only add workload node if not PodsOnly mode
			if !opts.PodsOnly {
				nodes[nodeKey] = &RundeckNode{
					NodeName:           nodeKey,
					Hostname:           "localhost",
					Tags:               allTags,
					OSFamily:           "kubernetes",
					NodeExecutor:       "local",
					FileCopier:         "local",
					Cluster:            opts.ClusterName,
					ClusterURL:         opts.ClusterURL,
					ClusterTokenSuffix: getTokenSuffix(opts),
					TargetType:         "statefulset",
					TargetValue:        stsName,
					TargetNamespace:    stsNS,
					WorkloadKind:       "StatefulSet",
					WorkloadName:       stsName,
					PodCount:           fmt.Sprintf("%d", replicas),
					HealthyPods:        fmt.Sprintf("%d", healthy),
					Healthy:            fmt.Sprintf("%t", healthy >= int(replicas)),
					ExtraAttributes:    extraAttrs,
				}
			}

			// Track for pod expansion (standalone statefulset)
			if opts.IncludePods {
				podInfos = append(podInfos, workloadPodInfo{
					workloadType:   TypeStatefulSet,
					workloadName:   stsName,
					workloadKey:    nodeKey,
					namespace:      stsNS,
					selectorLabels: selectorLabels,
					labels:         labels,
					annotations:    annotations,
				})
			}
		}
	}

	return podInfos, nil
}

func discoverDeployments(ctx context.Context, client dynamic.Interface, opts DiscoverOptions, listOpts metav1.ListOptions, filter *Filter, nodes map[string]*RundeckNode, helmReleases map[string]*helmInfo) ([]workloadPodInfo, error) {
	var deployList *unstructured.UnstructuredList
	var err error
	var podInfos []workloadPodInfo

	if opts.AllNamespaces {
		deployList, err = client.Resource(deployGVR).Namespace("").List(ctx, listOpts)
	} else {
		deployList, err = client.Resource(deployGVR).Namespace(opts.Namespace).List(ctx, listOpts)
	}
	if err != nil {
		return nil, err
	}

	for _, deploy := range deployList.Items {
		deployName := deploy.GetName()
		deployNS := deploy.GetNamespace()
		labels := deploy.GetLabels()
		annotations := deploy.GetAnnotations()
		selectorLabels := getSelectorLabels(&deploy)
		replicas, _, _ := unstructured.NestedInt64(deploy.Object, "spec", "replicas")
		if replicas == 0 {
			replicas = 1
		}

		healthy := countHealthyPods(ctx, client, deployNS, selectorLabels)

		// Apply filters (exclude labels, operator exclusion)
		if filter.ShouldExcludeByLabels(labels) {
			continue
		}
		if filter.ShouldExcludeOperator(deployName, labels) {
			continue
		}

		helmRelease := labels["app.kubernetes.io/instance"]
		if helmRelease != "" {
			key := helmRelease + "@" + deployNS
			if existing, ok := helmReleases[key]; ok {
				existing.totalPods += int(replicas)
				existing.healthyPods += healthy
			} else {
				helmReleases[key] = &helmInfo{
					release:      helmRelease,
					namespace:    deployNS,
					workloadKind: "Deployment",
					workloadName: deployName,
					totalPods:    int(replicas),
					healthyPods:  healthy,
					labels:       labels,
					annotations:  annotations,
				}
			}
			// Track for pod expansion (helm release)
			if opts.IncludePods {
				podInfos = append(podInfos, workloadPodInfo{
					workloadType:   TypeHelmRelease,
					workloadName:   helmRelease,
					workloadKey:    "", // Will be set later when helm node is created
					namespace:      deployNS,
					selectorLabels: selectorLabels,
					labels:         labels,
					annotations:    annotations,
				})
			}
		} else {
			// Apply health filter for non-Helm workloads
			if filter.ShouldExcludeByHealth(healthy, int(replicas)) {
				continue
			}

			// Build extra tags and attributes
			baseTags := joinTags("deployment", deployNS, opts.ClusterName)
			extraTags := buildExtraTags(opts, labels)
			allTags := mergeTagStrings(baseTags, extraTags)
			extraAttrs := buildExtraAttributes(opts, labels, annotations)

			nodeKey := makeNodeKey(opts.ClusterName, "deploy", deployName, deployNS)

			// Only add workload node if not PodsOnly mode
			if !opts.PodsOnly {
				nodes[nodeKey] = &RundeckNode{
					NodeName:           nodeKey,
					Hostname:           "localhost",
					Tags:               allTags,
					OSFamily:           "kubernetes",
					NodeExecutor:       "local",
					FileCopier:         "local",
					Cluster:            opts.ClusterName,
					ClusterURL:         opts.ClusterURL,
					ClusterTokenSuffix: getTokenSuffix(opts),
					TargetType:         "deployment",
					TargetValue:        deployName,
					TargetNamespace:    deployNS,
					WorkloadKind:       "Deployment",
					WorkloadName:       deployName,
					PodCount:           fmt.Sprintf("%d", replicas),
					HealthyPods:        fmt.Sprintf("%d", healthy),
					Healthy:            fmt.Sprintf("%t", healthy >= int(replicas)),
					ExtraAttributes:    extraAttrs,
				}
			}

			// Track for pod expansion (standalone deployment)
			if opts.IncludePods {
				podInfos = append(podInfos, workloadPodInfo{
					workloadType:   TypeDeployment,
					workloadName:   deployName,
					workloadKey:    nodeKey,
					namespace:      deployNS,
					selectorLabels: selectorLabels,
					labels:         labels,
					annotations:    annotations,
				})
			}
		}
	}

	return podInfos, nil
}

// getSelectorLabels extracts spec.selector.matchLabels from a workload.
func getSelectorLabels(obj *unstructured.Unstructured) map[string]string {
	labels, found, err := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
	if err != nil || !found {
		return nil
	}
	return labels
}

// countHealthyPods counts running pods matching the given selector labels.
func countHealthyPods(ctx context.Context, client dynamic.Interface, namespace string, selectorLabels map[string]string) int {
	if len(selectorLabels) == 0 {
		return 0
	}

	parts := make([]string, 0, len(selectorLabels))
	for k, v := range selectorLabels {
		parts = append(parts, k+"="+v)
	}
	selector := strings.Join(parts, ",")

	pods, err := client.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return 0
	}

	healthy := 0
	for _, pod := range pods.Items {
		phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
		if phase == "Running" {
			healthy++
		}
	}
	return healthy
}

// joinTags creates a comma-separated tag string for Rundeck nodes.
func joinTags(workloadType, namespace, cluster string) string {
	if cluster != "" {
		return workloadType + "," + namespace + "," + cluster
	}
	return workloadType + "," + namespace
}

// makeNodeKey creates a unique node key, optionally prefixed with cluster name.
func makeNodeKey(cluster, workloadType, name, namespace string) string {
	baseKey := fmt.Sprintf("%s:%s@%s", workloadType, name, namespace)
	if cluster != "" {
		return cluster + "/" + baseKey
	}
	return baseKey
}

// getTokenSuffix returns the effective token path suffix.
func getTokenSuffix(opts DiscoverOptions) string {
	if opts.ClusterTokenSuffix != "" {
		return opts.ClusterTokenSuffix
	}
	if opts.DefaultTokenSuffix != "" {
		return opts.DefaultTokenSuffix
	}
	return DefaultTokenSuffix
}

// workloadPodInfo tracks a workload's pod selector for pod expansion.
type workloadPodInfo struct {
	workloadType   string            // "statefulset", "deployment", or "helm-release"
	workloadName   string            // Name of the workload
	workloadKey    string            // Full node key (e.g., "cluster/sts:name@ns")
	namespace      string            // Namespace
	selectorLabels map[string]string // Pod selector labels
	labels         map[string]string // Workload labels (for extra tags/attrs)
	annotations    map[string]string // Workload annotations
}

// discoverPodsForWorkload discovers pods belonging to a workload and returns them as nodes.
func discoverPodsForWorkload(ctx context.Context, client dynamic.Interface, opts DiscoverOptions, filter *Filter, info workloadPodInfo) ([]*RundeckNode, error) {
	if len(info.selectorLabels) == 0 {
		return nil, nil
	}

	// Build label selector string
	parts := make([]string, 0, len(info.selectorLabels))
	for k, v := range info.selectorLabels {
		parts = append(parts, k+"="+v)
	}
	selector := strings.Join(parts, ",")

	pods, err := client.Resource(podGVR).Namespace(info.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}

	var podNodes []*RundeckNode
	podCount := 0

	for _, pod := range pods.Items {
		// Check max pods limit
		if opts.MaxPodsPerWorkload > 0 && podCount >= opts.MaxPodsPerWorkload {
			break
		}

		podName := pod.GetName()
		podNS := pod.GetNamespace()
		podLabels := pod.GetLabels()
		podAnnotations := pod.GetAnnotations()

		// Extract pod status info
		phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
		podIP, _, _ := unstructured.NestedString(pod.Object, "status", "podIP")
		hostIP, _, _ := unstructured.NestedString(pod.Object, "status", "hostIP")
		nodeName, _, _ := unstructured.NestedString(pod.Object, "spec", "nodeName")

		// Calculate container status
		containerStatuses, _, _ := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
		readyContainers := 0
		totalRestarts := 0
		for _, cs := range containerStatuses {
			if csMap, ok := cs.(map[string]interface{}); ok {
				if ready, ok := csMap["ready"].(bool); ok && ready {
					readyContainers++
				}
				if restartCount, ok := csMap["restartCount"].(int64); ok {
					totalRestarts += int(restartCount)
				}
			}
		}

		// Get container count from spec
		containers, _, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
		containerCount := len(containers)

		// Check if pod is ready (all containers ready)
		isReady := readyContainers == containerCount && containerCount > 0

		// Apply pod-specific filters
		if !filter.ShouldIncludePod(podName, phase, isReady, podLabels) {
			continue
		}

		// Build tags - include "pod" type and parent info
		baseTags := joinTags("pod", podNS, opts.ClusterName)
		// Add parent type as tag
		baseTags += "," + info.workloadType
		extraTags := buildExtraTags(opts, podLabels)
		allTags := mergeTagStrings(baseTags, extraTags)

		// Build extra attributes from pod labels/annotations
		extraAttrs := buildExtraAttributes(opts, podLabels, podAnnotations)

		nodeKey := makeNodeKey(opts.ClusterName, "pod", podName, podNS)
		podNode := &RundeckNode{
			NodeName:           nodeKey,
			Hostname:           "localhost",
			Tags:               allTags,
			OSFamily:           "kubernetes",
			NodeExecutor:       "local",
			FileCopier:         "local",
			Cluster:            opts.ClusterName,
			ClusterURL:         opts.ClusterURL,
			ClusterTokenSuffix: getTokenSuffix(opts),
			TargetType:         TypePod,
			TargetValue:        podName,
			TargetNamespace:    podNS,
			WorkloadKind:       "Pod",
			WorkloadName:       podName,
			PodCount:           "1",
			HealthyPods:        boolToHealthyCount(phase == "Running" && isReady),
			Healthy:            fmt.Sprintf("%t", phase == "Running" && isReady),
			PodInfo: &PodInfo{
				PodIP:           podIP,
				HostIP:          hostIP,
				K8sNodeName:     nodeName,
				Phase:           phase,
				Ready:           isReady,
				Restarts:        totalRestarts,
				ContainerCount:  containerCount,
				ReadyContainers: readyContainers,
				ParentType:      info.workloadType,
				ParentName:      info.workloadName,
				ParentNodename:  info.workloadKey,
			},
			ExtraAttributes: extraAttrs,
		}

		podNodes = append(podNodes, podNode)
		podCount++
	}

	return podNodes, nil
}

// boolToHealthyCount converts a boolean health status to "1" or "0".
func boolToHealthyCount(healthy bool) string {
	if healthy {
		return "1"
	}
	return "0"
}
