// kubectl-rundeck-nodes discovers Kubernetes workloads and outputs them
// as Rundeck resource model JSON for use as a script-based node source.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bluecontainer/kubectl-rundeck-nodes/pkg/nodes"
)

var (
	version = "dev"

	// Flags
	kubeconfig         string
	server             string
	token              string
	insecure           bool
	namespace          string
	allNamespaces      bool
	labelSelector      string
	clusterName        string
	clusterURL         string
	clusterTokenSuffix string
	defaultTokenSuffix string
	output             string

	// Phase 1: Core Filtering
	types           []string
	excludeTypes    []string
	excludeLabels   []string
	excludeOperator bool
	healthyOnly     bool
	unhealthyOnly   bool

	// Phase 2: Pattern Matching
	namePatterns             []string
	excludePatterns          []string
	excludeNamespaces        []string
	namespacePatterns        []string
	excludeNamespacePatterns []string

	// Phase 4: Output Customization
	addTags              []string
	labelsAsTags         []string
	labelAttributes      []string
	annotationAttributes []string

	// Phase 5: Pod Discovery
	includePods        bool
	podsOnly           bool
	podStatus          []string
	podNamePatterns    []string
	podReadyOnly       bool
	maxPodsPerWorkload int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "kubectl-rundeck-nodes",
	Short: "Discover Kubernetes workloads as Rundeck nodes",
	Long: `Discover Kubernetes workloads (Helm releases, StatefulSets, Deployments) and
output them as Rundeck resource model JSON. This command is designed to be used
as a Rundeck script-based resource model source.

Each discovered workload becomes a Rundeck node with attributes that map to
kubectl plugin --target-* flags:

  targetType:      helm-release, statefulset, deployment, or pod
  targetValue:     the workload or release name (or pod name)
  targetNamespace: the workload's namespace
  workloadKind:    StatefulSet, Deployment, or Pod
  workloadName:    the underlying workload name
  podCount:        total pod count
  healthyPods:     running pod count

Pod nodes (when --include-pods is used) have additional attributes:
  parentType:      parent workload type (statefulset, deployment, helm-release)
  parentName:      parent workload name
  parentNodename:  full nodename of the parent workload
  podIP:           pod's cluster IP
  hostIP:          IP of the node where pod runs
  k8sNode:         Kubernetes node name
  phase:           pod phase (Running, Pending, etc.)
  ready:           whether all containers are ready
  restarts:        total container restart count

Examples:
  # Discover workloads in the default namespace
  kubectl rundeck-nodes

  # Discover workloads across all namespaces
  kubectl rundeck-nodes --all-namespaces

  # Filter by label selector
  kubectl rundeck-nodes -l app.kubernetes.io/part-of=myapp

  # Multi-cluster setup with token suffix
  kubectl rundeck-nodes --cluster-name=prod --cluster-token-suffix=clusters/prod/token

  # Filter by workload type
  kubectl rundeck-nodes --types=statefulset,helm-release

  # Exclude operator workloads
  kubectl rundeck-nodes --exclude-operator

  # Only healthy workloads
  kubectl rundeck-nodes --healthy-only

  # Include individual pods for each workload
  kubectl rundeck-nodes --include-pods

  # Only pods (no workload nodes)
  kubectl rundeck-nodes --pods-only

  # Only running pods, first replica only
  kubectl rundeck-nodes --pods-only --pod-status=Running --pod-name-pattern="*-0"

  # Limit pods per workload to avoid node explosion
  kubectl rundeck-nodes --include-pods --max-pods-per-workload=5`,
	Version: version,
	RunE:    run,
}

func init() {
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	rootCmd.Flags().StringVar(&server, "server", "", "Kubernetes API server URL")
	rootCmd.Flags().StringVar(&token, "token", "", "Bearer token for authentication")
	rootCmd.Flags().BoolVar(&insecure, "insecure-skip-tls-verify", false, "Skip TLS certificate verification")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to discover workloads in")
	rootCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Discover workloads across all namespaces")
	rootCmd.Flags().StringVarP(&labelSelector, "selector", "l", "", "Label selector to filter workloads")
	rootCmd.Flags().StringVar(&clusterName, "cluster-name", "", "Cluster identifier for multi-cluster discovery")
	rootCmd.Flags().StringVar(&clusterURL, "cluster-url", "", "Cluster API URL to embed in node attributes")
	rootCmd.Flags().StringVar(&clusterTokenSuffix, "cluster-token-suffix", "", "Key Storage path suffix for cluster token")
	rootCmd.Flags().StringVar(&defaultTokenSuffix, "default-token-suffix", nodes.DefaultTokenSuffix, "Default Key Storage path suffix")
	rootCmd.Flags().StringVarP(&output, "output", "o", "json", "Output format: json, yaml, table")

	// Phase 1: Core Filtering flags
	rootCmd.Flags().StringSliceVar(&types, "types", nil, "Only include these workload types (helm-release, statefulset, deployment)")
	rootCmd.Flags().StringSliceVar(&excludeTypes, "exclude-types", nil, "Exclude these workload types")
	rootCmd.Flags().StringSliceVar(&excludeLabels, "exclude-labels", nil, "Exclude workloads matching these label selectors")
	rootCmd.Flags().BoolVar(&excludeOperator, "exclude-operator", false, "Exclude operator controller-manager workloads")
	rootCmd.Flags().BoolVar(&healthyOnly, "healthy-only", false, "Only include workloads with all pods running")
	rootCmd.Flags().BoolVar(&unhealthyOnly, "unhealthy-only", false, "Only include workloads with some pods not running")

	// Phase 2: Pattern Matching flags
	rootCmd.Flags().StringSliceVar(&namePatterns, "name-pattern", nil, "Only include workloads matching these glob patterns (e.g., myapp-*)")
	rootCmd.Flags().StringSliceVar(&excludePatterns, "exclude-pattern", nil, "Exclude workloads matching these glob patterns")
	rootCmd.Flags().StringSliceVar(&excludeNamespaces, "exclude-namespaces", nil, "Exclude workloads in these namespaces")
	rootCmd.Flags().StringSliceVar(&namespacePatterns, "namespace-pattern", nil, "Only include workloads in namespaces matching these patterns")
	rootCmd.Flags().StringSliceVar(&excludeNamespacePatterns, "exclude-namespace-pattern", nil, "Exclude workloads in namespaces matching these patterns")

	// Phase 4: Output Customization flags
	rootCmd.Flags().StringSliceVar(&addTags, "add-tags", nil, "Add custom tags to all nodes (e.g., env:prod,team:platform)")
	rootCmd.Flags().StringSliceVar(&labelsAsTags, "labels-as-tags", nil, "Convert these Kubernetes labels to Rundeck tags (e.g., app.kubernetes.io/name)")
	rootCmd.Flags().StringSliceVar(&labelAttributes, "label-attributes", nil, "Add these Kubernetes labels as node attributes (e.g., app.kubernetes.io/version)")
	rootCmd.Flags().StringSliceVar(&annotationAttributes, "annotation-attributes", nil, "Add these Kubernetes annotations as node attributes")

	// Phase 5: Pod Discovery flags
	rootCmd.Flags().BoolVar(&includePods, "include-pods", false, "Include individual pods as nodes (in addition to workloads)")
	rootCmd.Flags().BoolVar(&podsOnly, "pods-only", false, "Only return pod nodes, excluding workload-level nodes (implies --include-pods)")
	rootCmd.Flags().StringSliceVar(&podStatus, "pod-status", nil, "Only include pods with these phases (e.g., Running, Pending)")
	rootCmd.Flags().StringSliceVar(&podNamePatterns, "pod-name-pattern", nil, "Only include pods matching these glob patterns (e.g., *-0 for first StatefulSet replica)")
	rootCmd.Flags().BoolVar(&podReadyOnly, "pod-ready-only", false, "Only include pods where all containers are ready")
	rootCmd.Flags().IntVar(&maxPodsPerWorkload, "max-pods-per-workload", 0, "Maximum number of pod nodes per workload (0 = unlimited)")
}

func run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, ns, err := buildClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Use namespace from flag or kubeconfig
	if namespace != "" {
		ns = namespace
	}
	if ns == "" {
		ns = "default"
	}

	opts := nodes.DiscoverOptions{
		Namespace:          ns,
		AllNamespaces:      allNamespaces,
		LabelSelector:      labelSelector,
		ClusterName:        clusterName,
		ClusterURL:         clusterURL,
		ClusterTokenSuffix: clusterTokenSuffix,
		DefaultTokenSuffix: defaultTokenSuffix,
		// Phase 1: Core Filtering
		Types:           types,
		ExcludeTypes:    excludeTypes,
		ExcludeLabels:   excludeLabels,
		ExcludeOperator: excludeOperator,
		HealthyOnly:     healthyOnly,
		UnhealthyOnly:   unhealthyOnly,
		// Phase 2: Pattern Matching
		NamePatterns:             namePatterns,
		ExcludePatterns:          excludePatterns,
		ExcludeNamespaces:        excludeNamespaces,
		NamespacePatterns:        namespacePatterns,
		ExcludeNamespacePatterns: excludeNamespacePatterns,
		// Phase 4: Output Customization
		AddTags:              addTags,
		LabelsAsTags:         labelsAsTags,
		LabelAttributes:      labelAttributes,
		AnnotationAttributes: annotationAttributes,
		// Phase 5: Pod Discovery
		IncludePods:        includePods,
		PodsOnly:           podsOnly,
		PodStatusFilter:    podStatus,
		PodNamePatterns:    podNamePatterns,
		PodReadyOnly:       podReadyOnly,
		MaxPodsPerWorkload: maxPodsPerWorkload,
	}

	discovered, err := nodes.Discover(ctx, client, opts)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	format := nodes.OutputFormat(output)
	return nodes.Write(os.Stdout, discovered, format)
}

func buildClient() (dynamic.Interface, string, error) {
	var config *rest.Config
	var ns string
	var err error

	if server != "" && token != "" {
		// Direct connection with server/token
		config = &rest.Config{
			Host:        server,
			BearerToken: token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: insecure,
			},
		}
		ns = namespace
	} else {
		// Load from kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfig != "" {
			loadingRules.ExplicitPath = kubeconfig
		}

		configOverrides := &clientcmd.ConfigOverrides{}
		if namespace != "" {
			configOverrides.Context.Namespace = namespace
		}

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, "", err
		}

		ns, _, err = clientConfig.Namespace()
		if err != nil {
			ns = "default"
		}
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, "", err
	}

	return client, ns, nil
}
