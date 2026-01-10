// Package diagnostics provides comprehensive cluster diagnostics functionality.
package diagnostics

import (
	"context"
	"fmt"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
	"k8s.io/client-go/kubernetes"
)

// Result represents the result of diagnostics
type Result struct {
	Summary      Summary
	NodeIssues   []NodeIssue
	PodIssues    []PodIssue
	SystemIssues []SystemIssue
	EventIssues  []EventIssue
}

// Summary provides an overview of issues found
type Summary struct {
	TotalIssues   int
	CriticalCount int
	WarningCount  int
	InfoCount     int
}

// NodeIssue represents an issue with a node
type NodeIssue struct {
	Node     string
	Severity string // Critical, Warning, Info
	Type     string
	Message  string
}

// PodIssue represents an issue with a pod
type PodIssue struct {
	Pod       string
	Namespace string
	Severity  string
	Type      string
	Message   string
	Restarts  int32
}

// SystemIssue represents a system-level issue
type SystemIssue struct {
	Component string
	Severity  string
	Type      string
	Message   string
}

// EventIssue represents an issue detected from cluster events
type EventIssue struct {
	Type      string // Warning, Error
	Reason    string
	Message   string
	Object    string
	Namespace string
	Severity  string
	Count     int32
}

// RunDiagnostics performs comprehensive cluster diagnostics
func RunDiagnostics(ctx context.Context, clientset kubernetes.Interface, namespace string) (*Result, error) {
	result := &Result{
		NodeIssues:   []NodeIssue{},
		PodIssues:    []PodIssue{},
		SystemIssues: []SystemIssue{},
		EventIssues:  []EventIssue{},
	}

	// Check nodes
	nodes, err := healthcheck.CheckNodes(ctx, clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to check nodes: %w", err)
	}

	for _, node := range nodes {
		issues := diagnoseNode(&node)
		result.NodeIssues = append(result.NodeIssues, issues...)
	}

	// Check pods
	pods, err := healthcheck.CheckPods(ctx, clientset, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check pods: %w", err)
	}

	for _, pod := range pods.ProblemPods {
		issue := diagnosePod(&pod)
		result.PodIssues = append(result.PodIssues, issue)
	}

	// Check components
	components, err := healthcheck.CheckComponents(ctx, clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to check components: %w", err)
	}

	for _, comp := range components {
		if issue := diagnoseComponent(&comp); issue != nil {
			result.SystemIssues = append(result.SystemIssues, *issue)
		}
	}

	// Check events
	eventStatus, err := healthcheck.CheckEvents(ctx, clientset, namespace)
	if err != nil {
		// Log error but continue with other diagnostics
		fmt.Printf("Warning: failed to check events: %v\n", err)
	} else if eventStatus != nil {
		for _, event := range eventStatus.Events {
			result.EventIssues = append(result.EventIssues, EventIssue{
				Type:      event.Type,
				Reason:    event.Reason,
				Message:   event.Message,
				Object:    event.Object,
				Namespace: event.Namespace,
				Severity:  mapEventSeverity(event.Type),
				Count:     event.Count,
			})
		}
	}

	// Calculate summary
	result.Summary = calculateSummary(result)

	return result, nil
}

// diagnoseNode analyzes a node and returns issues
func diagnoseNode(node *healthcheck.NodeStatus) []NodeIssue {
	issues := make([]NodeIssue, 0, len(node.Issues)+1)

	// Not ready is critical
	if node.Status == "NotReady" {
		issues = append(issues, NodeIssue{
			Node:     node.Name,
			Severity: "Critical",
			Type:     "NodeNotReady",
			Message:  "Node is not in Ready state",
		})
	}

	// Check for specific issues
	for _, issue := range node.Issues {
		severity := "Warning"
		issueType := "NodePressure"

		// Memory and disk pressure are critical
		if issue == "Memory pressure detected" || issue == "Disk pressure detected" {
			severity = "Critical"
		}

		// Cordoned node is info
		if issue == "Node is cordoned (unschedulable)" {
			severity = "Info"
			issueType = "NodeCordoned"
		}

		issues = append(issues, NodeIssue{
			Node:     node.Name,
			Severity: severity,
			Type:     issueType,
			Message:  issue,
		})
	}

	return issues
}

// diagnosePod analyzes a problem pod and returns an issue
func diagnosePod(pod *healthcheck.ProblemPod) PodIssue {
	issue := PodIssue{
		Pod:       pod.Name,
		Namespace: pod.Namespace,
		Type:      pod.Reason,
		Message:   pod.Message,
		Restarts:  pod.Restarts,
	}

	// Determine severity based on reason
	switch pod.Reason {
	case "CrashLoopBackOff":
		issue.Severity = "Critical"
		issue.Type = "CrashLoopBackOff"
	case "ImagePullBackOff", "ErrImagePull":
		issue.Severity = "Critical"
		issue.Type = "ImagePullError"
	case "CreateContainerError", "RunContainerError":
		issue.Severity = "Critical"
		issue.Type = "ContainerError"
	case "Pending":
		issue.Severity = "Warning"
		issue.Type = "PodPending"
	case "Failed":
		issue.Severity = "Critical"
		issue.Type = "PodFailed"
	default:
		issue.Severity = "Warning"
		switch {
		case pod.Restarts > 10:
			issue.Severity = "Critical"
			issue.Type = "HighRestartCount"
		case pod.Restarts > 5:
			issue.Type = "FrequentRestarts"
		}
	}

	if issue.Message == "" {
		issue.Message = fmt.Sprintf("Pod has %d restarts", pod.Restarts)
	}

	return issue
}

// diagnoseComponent analyzes a component and returns an issue if unhealthy
func diagnoseComponent(comp *healthcheck.ComponentStatus) *SystemIssue {
	if comp.Status == "Healthy" {
		return nil
	}

	return &SystemIssue{
		Component: comp.Name,
		Severity:  "Critical",
		Type:      "ComponentUnhealthy",
		Message:   comp.Message,
	}
}

// calculateSummary calculates the summary statistics
func calculateSummary(result *Result) Summary {
	summary := Summary{}

	// Count node issues
	for _, issue := range result.NodeIssues {
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

	// Count pod issues
	for _, issue := range result.PodIssues {
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

	// Count system issues
	for _, issue := range result.SystemIssues {
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

	// Count event issues
	for _, issue := range result.EventIssues {
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

// mapEventSeverity maps Kubernetes event type to diagnostics severity
func mapEventSeverity(eventType string) string {
	switch eventType {
	case "Warning":
		return "Warning"
	case "Error":
		return "Critical"
	default:
		return "Info"
	}
}
