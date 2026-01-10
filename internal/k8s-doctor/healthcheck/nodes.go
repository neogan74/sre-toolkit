package healthcheck

import (
	"context"
	"fmt"

	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NodeStatus represents the health status of a node
type NodeStatus struct {
	Name        string
	Status      string // Ready, NotReady, Unknown
	Conditions  []NodeCondition
	Roles       []string
	Version     string
	CPUUsage    string
	MemoryUsage string
	Issues      []string
}

// NodeMetricsList represents the response from metrics API
type NodeMetricsList struct {
	Kind       string        `json:"kind"`
	APIVersion string        `json:"apiVersion"`
	Items      []NodeMetrics `json:"items"`
}

// NodeMetrics represents metrics for a single node
type NodeMetrics struct {
	Metadata metav1.ObjectMeta   `json:"metadata"`
	Usage    corev1.ResourceList `json:"usage"`
}

// NodeCondition represents a node condition
type NodeCondition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// CheckNodes checks the health status of all nodes in the cluster
func CheckNodes(ctx context.Context, clientset kubernetes.Interface) ([]NodeStatus, error) {
	// Get API server version for compatibility check
	serverVersion := "unknown"
	if versionInfo, err := clientset.Discovery().ServerVersion(); err == nil {
		serverVersion = versionInfo.GitVersion
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Try to get node metrics
	metricsMap, _ := getNodeMetrics(ctx, clientset)

	statuses := make([]NodeStatus, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		usage := corev1.ResourceList{}
		if metricsMap != nil {
			if m, ok := metricsMap[node.Name]; ok {
				usage = m
			}
		}
		status := analyzeNode(&node, serverVersion, usage)
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// getNodeMetrics fetches node metrics from the K8s Metrics API
func getNodeMetrics(ctx context.Context, client kubernetes.Interface) (map[string]corev1.ResourceList, error) {
	rc := client.CoreV1().RESTClient()
	if rc == nil || (reflect.ValueOf(rc).Kind() == reflect.Ptr && reflect.ValueOf(rc).IsNil()) {
		return nil, fmt.Errorf("REST client is nil")
	}

	data, err := rc.Get().AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").DoRaw(ctx)
	if err != nil {
		return nil, err
	}

	var metricsList NodeMetricsList
	if err := json.Unmarshal(data, &metricsList); err != nil {
		return nil, err
	}

	result := make(map[string]corev1.ResourceList)
	for _, m := range metricsList.Items {
		result[m.Metadata.Name] = m.Usage
	}
	return result, nil
}

// analyzeNode analyzes a single node and returns its status
func analyzeNode(node *corev1.Node, apiServerVersion string, usage corev1.ResourceList) NodeStatus {
	status := NodeStatus{
		Name:        node.Name,
		Status:      "Unknown",
		Conditions:  []NodeCondition{},
		Roles:       getRoles(node),
		Version:     node.Status.NodeInfo.KubeletVersion,
		CPUUsage:    "N/A",
		MemoryUsage: "N/A",
		Issues:      []string{},
	}

	// Check version compatibility
	if apiServerVersion != "unknown" && status.Version != "" {
		apiVersion, err1 := ParseVersion(apiServerVersion)
		kubeletVersion, err2 := ParseVersion(status.Version)
		if err1 == nil && err2 == nil {
			if skewIssue := GetVersionSkewDescription(kubeletVersion, apiVersion, "Kubelet"); skewIssue != "" {
				status.Issues = append(status.Issues, skewIssue)
			}
		}
	}

	// Check node conditions
	for _, condition := range node.Status.Conditions {
		nc := NodeCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		}
		status.Conditions = append(status.Conditions, nc)

		// Check for Ready status
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				status.Status = "Ready"
			} else {
				status.Status = "NotReady"
				status.Issues = append(status.Issues, fmt.Sprintf("Node is not ready: %s", condition.Message))
			}
		}

		// Check for pressure conditions
		if condition.Status == corev1.ConditionTrue {
			switch condition.Type {
			case corev1.NodeMemoryPressure:
				status.Issues = append(status.Issues, "Memory pressure detected")
			case corev1.NodeDiskPressure:
				status.Issues = append(status.Issues, "Disk pressure detected")
			case corev1.NodePIDPressure:
				status.Issues = append(status.Issues, "PID pressure detected")
			case corev1.NodeNetworkUnavailable:
				status.Issues = append(status.Issues, "Network unavailable")
			}
		}
	}

	if node.Spec.Unschedulable {
		status.Issues = append(status.Issues, "Node is cordoned (unschedulable)")
	}

	// Calculate resource usage if available
	if len(usage) > 0 {
		// CPU Usage
		cpuUsage := usage.Cpu().MilliValue()
		cpuAllocatable := node.Status.Allocatable.Cpu().MilliValue()
		if cpuAllocatable > 0 {
			percent := float64(cpuUsage) / float64(cpuAllocatable) * 100
			status.CPUUsage = fmt.Sprintf("%.0f%%", percent)
			if percent > 90 {
				status.Issues = append(status.Issues, fmt.Sprintf("High CPU usage: %.0f%% (>90%%)", percent))
			} else if percent > 80 {
				status.Issues = append(status.Issues, fmt.Sprintf("Elevated CPU usage: %.0f%% (>80%%)", percent))
			}
		}

		// Memory Usage
		memUsage := usage.Memory().Value()
		memAllocatable := node.Status.Allocatable.Memory().Value()
		if memAllocatable > 0 {
			percent := float64(memUsage) / float64(memAllocatable) * 100
			status.MemoryUsage = fmt.Sprintf("%.0f%%", percent)
			if percent > 90 {
				status.Issues = append(status.Issues, fmt.Sprintf("High Memory usage: %.0f%% (>90%%)", percent))
			} else if percent > 80 {
				status.Issues = append(status.Issues, fmt.Sprintf("Elevated Memory usage: %.0f%% (>80%%)", percent))
			}
		}
	}

	return status
}

// getRoles extracts roles from node labels
func getRoles(node *corev1.Node) []string {
	roles := []string{}
	for label := range node.Labels {
		switch label {
		case "node-role.kubernetes.io/master", "node-role.kubernetes.io/control-plane":
			roles = append(roles, "control-plane")
		case "node-role.kubernetes.io/worker":
			roles = append(roles, "worker")
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "worker")
	}
	return roles
}
