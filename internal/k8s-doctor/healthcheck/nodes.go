package healthcheck

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NodeStatus represents the health status of a node
type NodeStatus struct {
	Name       string
	Status     string // Ready, NotReady, Unknown
	Conditions []NodeCondition
	Roles      []string
	Version    string
	Issues     []string
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

	statuses := make([]NodeStatus, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		status := analyzeNode(&node, serverVersion)
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// analyzeNode analyzes a single node and returns its status
func analyzeNode(node *corev1.Node, apiServerVersion string) NodeStatus {
	status := NodeStatus{
		Name:       node.Name,
		Status:     "Unknown",
		Conditions: []NodeCondition{},
		Roles:      getRoles(node),
		Version:    node.Status.NodeInfo.KubeletVersion,
		Issues:     []string{},
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

	// Check if node is schedulable
	if node.Spec.Unschedulable {
		status.Issues = append(status.Issues, "Node is cordoned (unschedulable)")
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
