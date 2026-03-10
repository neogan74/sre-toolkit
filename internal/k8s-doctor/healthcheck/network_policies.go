package healthcheck

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NetworkPoliciesStatus represents the health status of network policies
type NetworkPoliciesStatus struct {
	TotalNamespaces int
	TotalPolicies   int
	Issues          []NetworkPolicyIssue
}

// NetworkPolicyIssue represents a namespace without network policies
type NetworkPolicyIssue struct {
	Namespace string
	Severity  string
	Message   string
}

// CheckNetworkPolicies audits namespaces for the presence of NetworkPolicies.
// It skips system namespaces like kube-system, kube-public, etc.
func CheckNetworkPolicies(ctx context.Context, clientset kubernetes.Interface, namespace string) (*NetworkPoliciesStatus, error) {
	status := &NetworkPoliciesStatus{
		Issues: []NetworkPolicyIssue{},
	}

	var namespacesToCheck []string

	// If a specific namespace is provided, check only that one
	if namespace != "" {
		namespacesToCheck = []string{namespace}
	} else {
		// List all namespaces
		nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list namespaces: %w", err)
		}

		for _, ns := range nsList.Items {
			// Skip system namespaces
			if isSystemNamespace(ns.Name) {
				continue
			}
			namespacesToCheck = append(namespacesToCheck, ns.Name)
		}
	}

	status.TotalNamespaces = len(namespacesToCheck)

	for _, ns := range namespacesToCheck {
		policies, err := clientset.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list network policies in namespace %s: %w", ns, err)
		}

		policyCount := len(policies.Items)
		status.TotalPolicies += policyCount

		if policyCount == 0 {
			status.Issues = append(status.Issues, NetworkPolicyIssue{
				Namespace: ns,
				Severity:  "Warning",
				Message:   fmt.Sprintf("Namespace %s has no NetworkPolicies defined", ns),
			})
		}
	}

	return status, nil
}

// isSystemNamespace checks if a namespace is a system namespace that should be excluded from the audit
func isSystemNamespace(namespace string) bool {
	systemNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"local-path-storage",
		"cert-manager",
		"ingress-nginx",
	}

	for _, sysNs := range systemNamespaces {
		if namespace == sysNs {
			return true
		}
	}

	return strings.HasPrefix(namespace, "kube-")
}
