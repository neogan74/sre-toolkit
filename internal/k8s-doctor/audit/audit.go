// Package audit provides security and best-practices audit functionality.
package audit

import (
	"context"
	"fmt"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
	"k8s.io/client-go/kubernetes"
)

// Result represents the result of an audit run.
type Result struct {
	Summary             Summary
	ResourceIssues      []ResourceIssue
	ProbeIssues         []ProbeIssue
	SecurityIssues      []SecurityIssue
	NetworkPolicyIssues []healthcheck.NetworkPolicyIssue
}

// Summary provides an overview of issues found.
type Summary struct {
	TotalIssues   int
	CriticalCount int
	WarningCount  int
	InfoCount     int
}

// ResourceIssue represents a resource configuration issue.
type ResourceIssue struct {
	Pod       string
	Namespace string
	Severity  string
	Message   string
}

// ProbeIssue represents a probe configuration issue.
type ProbeIssue struct {
	Pod       string
	Namespace string
	Severity  string
	Message   string
}

// SecurityIssue represents a security configuration issue.
type SecurityIssue struct {
	Pod       string
	Namespace string
	Severity  string
	Message   string
}

// RunAudit performs a namespace-scoped or cluster-wide audit.
func RunAudit(ctx context.Context, clientset kubernetes.Interface, namespace string) (*Result, error) {
	result := &Result{
		ResourceIssues:      []ResourceIssue{},
		ProbeIssues:         []ProbeIssue{},
		SecurityIssues:      []SecurityIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	pods, err := healthcheck.CheckPods(ctx, clientset, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check pods: %w", err)
	}

	for _, audit := range pods.ResourceAudit {
		for _, container := range audit.Containers {
			for _, issueMsg := range container.Issues {
				result.ResourceIssues = append(result.ResourceIssues, ResourceIssue{
					Pod:       audit.Pod,
					Namespace: audit.Namespace,
					Severity:  "Warning",
					Message:   fmt.Sprintf("Container %s: %s", container.Name, issueMsg),
				})
			}
		}
	}

	for _, audit := range pods.ProbeAudit {
		for _, container := range audit.Containers {
			for _, issueMsg := range container.Issues {
				result.ProbeIssues = append(result.ProbeIssues, ProbeIssue{
					Pod:       audit.Pod,
					Namespace: audit.Namespace,
					Severity:  "Warning",
					Message:   fmt.Sprintf("Container %s: %s", container.Name, issueMsg),
				})
			}
		}
	}

	for _, audit := range pods.SecurityAudit {
		for _, container := range audit.Containers {
			for _, issueMsg := range container.Issues {
				result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
					Pod:       audit.Pod,
					Namespace: audit.Namespace,
					Severity:  "Critical",
					Message:   fmt.Sprintf("Container %s: %s", container.Name, issueMsg),
				})
			}
		}
	}

	networkPolicies, err := healthcheck.CheckNetworkPolicies(ctx, clientset, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check network policies: %w", err)
	}
	result.NetworkPolicyIssues = networkPolicies.Issues

	result.Summary = calculateSummary(result)

	return result, nil
}

func calculateSummary(result *Result) Summary {
	summary := Summary{}

	for _, issue := range result.ResourceIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.ProbeIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.SecurityIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.NetworkPolicyIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	return summary
}
