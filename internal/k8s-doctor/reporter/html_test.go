package reporter

import (
	"bytes"
	"testing"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/audit"
	"github.com/neogan/sre-toolkit/internal/k8s-doctor/diagnostics"
	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderHealthCheckHTML(t *testing.T) {
	buf := &bytes.Buffer{}

	nodes := []healthcheck.NodeStatus{
		{
			Name:        "node-1",
			Status:      "Ready",
			CPUUsage:    "50%",
			MemoryUsage: "60%",
			Issues:      []string{},
		},
	}

	pods := &healthcheck.PodStatus{
		Total:       10,
		Running:     8,
		Pending:     2,
		Failed:      0,
		Succeeded:   0,
		Unknown:     0,
		ProblemPods: []healthcheck.ProblemPod{},
	}

	components := []healthcheck.ComponentStatus{
		{
			Name:    "etcd",
			Status:  "Healthy",
			Message: "OK",
		},
	}

	netPols := &healthcheck.NetworkPoliciesStatus{
		TotalNamespaces: 5,
		TotalPolicies:   10,
		Issues:          []healthcheck.NetworkPolicyIssue{},
	}

	err := renderHealthCheckHTML(buf, nodes, pods, components, netPols)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "node-1")
	assert.Contains(t, output, "Kubernetes Health Report")
}

func TestRenderDiagnosticsHTML(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &diagnostics.Result{
		Summary: diagnostics.Summary{
			TotalIssues:   1,
			CriticalCount: 1,
			WarningCount:  0,
			InfoCount:     0,
		},
		NodeIssues: []diagnostics.NodeIssue{
			{
				Node:     "node-1",
				Severity: "Critical",
				Type:     "NodeNotReady",
				Message:  "Node is not ready",
			},
		},
		PodIssues:           []diagnostics.PodIssue{},
		SystemIssues:        []diagnostics.SystemIssue{},
		EventIssues:         []diagnostics.EventIssue{},
		ResourceIssues:      []diagnostics.ResourceIssue{},
		ProbeIssues:         []diagnostics.ProbeIssue{},
		SecurityIssues:      []diagnostics.SecurityContextIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	err := renderDiagnosticsHTML(buf, result)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "node-1")
}

func TestRenderDiagnosticsHTMLEmptyResult(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &diagnostics.Result{
		Summary: diagnostics.Summary{
			TotalIssues:   0,
			CriticalCount: 0,
			WarningCount:  0,
			InfoCount:     0,
		},
		NodeIssues:          []diagnostics.NodeIssue{},
		PodIssues:           []diagnostics.PodIssue{},
		SystemIssues:        []diagnostics.SystemIssue{},
		EventIssues:         []diagnostics.EventIssue{},
		ResourceIssues:      []diagnostics.ResourceIssue{},
		ProbeIssues:         []diagnostics.ProbeIssue{},
		SecurityIssues:      []diagnostics.SecurityContextIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	err := renderDiagnosticsHTML(buf, result)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
}

func TestRenderAuditHTML(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &audit.Result{
		Summary: audit.Summary{
			TotalIssues:   1,
			CriticalCount: 1,
			WarningCount:  0,
			InfoCount:     0,
		},
		SecurityIssues: []audit.SecurityIssue{
			{
				Pod:       "pod-1",
				Namespace: "default",
				Severity:  "Critical",
				Message:   "Running as root",
			},
		},
		ResourceIssues:      []audit.ResourceIssue{},
		ProbeIssues:         []audit.ProbeIssue{},
		RBACIssues:          []audit.RBACIssue{},
		ResourceQuotaIssues: []audit.ResourceQuotaIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	err := renderAuditHTML(buf, result)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "pod-1")
}

func TestRenderAuditHTMLEmptyResult(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &audit.Result{
		Summary: audit.Summary{
			TotalIssues:   0,
			CriticalCount: 0,
			WarningCount:  0,
			InfoCount:     0,
		},
		SecurityIssues:      []audit.SecurityIssue{},
		ResourceIssues:      []audit.ResourceIssue{},
		ProbeIssues:         []audit.ProbeIssue{},
		RBACIssues:          []audit.RBACIssue{},
		ResourceQuotaIssues: []audit.ResourceQuotaIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	err := renderAuditHTML(buf, result)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
}

func TestRenderHealthCheckHTMLWithProblems(t *testing.T) {
	buf := &bytes.Buffer{}

	nodes := []healthcheck.NodeStatus{
		{
			Name:        "node-1",
			Status:      "NotReady",
			CPUUsage:    "95%",
			MemoryUsage: "90%",
			Issues:      []string{"Memory pressure detected"},
		},
	}

	pods := &healthcheck.PodStatus{
		Total:     10,
		Running:   7,
		Pending:   2,
		Failed:    1,
		Succeeded: 0,
		Unknown:   0,
		ProblemPods: []healthcheck.ProblemPod{
			{
				Name:      "crash-pod",
				Namespace: "default",
				Status:    "CrashLoopBackOff",
				Reason:    "CrashLoopBackOff",
				Restarts:  10,
			},
		},
	}

	components := []healthcheck.ComponentStatus{
		{
			Name:    "etcd",
			Status:  "Unhealthy",
			Message: "Connection failed",
		},
	}

	netPols := &healthcheck.NetworkPoliciesStatus{
		TotalNamespaces: 5,
		TotalPolicies:   5,
		Issues: []healthcheck.NetworkPolicyIssue{
			{
				Namespace: "default",
				Severity:  "Warning",
				Message:   "Missing network policy",
			},
		},
	}

	err := renderHealthCheckHTML(buf, nodes, pods, components, netPols)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "node-1")
	assert.Contains(t, output, "crash-pod")
	assert.Contains(t, output, "etcd")
}

func TestRenderDiagnosticsHTMLWithMultipleIssues(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &diagnostics.Result{
		Summary: diagnostics.Summary{
			TotalIssues:   5,
			CriticalCount: 2,
			WarningCount:  2,
			InfoCount:     1,
		},
		NodeIssues: []diagnostics.NodeIssue{
			{
				Node:     "node-1",
				Severity: "Critical",
				Type:     "NodeNotReady",
				Message:  "Node is not ready",
			},
		},
		PodIssues: []diagnostics.PodIssue{
			{
				Pod:       "pod-1",
				Namespace: "default",
				Severity:  "Critical",
				Type:      "CrashLoopBackOff",
				Restarts:  10,
			},
		},
		SystemIssues: []diagnostics.SystemIssue{
			{
				Component: "etcd",
				Severity:  "Warning",
				Type:      "ComponentUnhealthy",
				Message:   "Connection refused",
			},
		},
		EventIssues: []diagnostics.EventIssue{
			{
				Object:    "pod-2",
				Namespace: "default",
				Severity:  "Warning",
				Reason:    "FailedScheduling",
				Message:   "Pod could not be scheduled",
				Count:     3,
			},
		},
		ResourceIssues: []diagnostics.ResourceIssue{
			{
				Pod:       "pod-3",
				Namespace: "default",
				Severity:  "Info",
				Message:   "No resource limits set",
			},
		},
		ProbeIssues:         []diagnostics.ProbeIssue{},
		SecurityIssues:      []diagnostics.SecurityContextIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	err := renderDiagnosticsHTML(buf, result)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
	// Check for presence of multiple issue types
	assert.Contains(t, output, "node-1")
	assert.Contains(t, output, "pod-1")
	assert.Contains(t, output, "etcd")
}

func TestRenderAuditHTMLWithMultipleIssues(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &audit.Result{
		Summary: audit.Summary{
			TotalIssues:   4,
			CriticalCount: 2,
			WarningCount:  2,
			InfoCount:     0,
		},
		SecurityIssues: []audit.SecurityIssue{
			{
				Pod:       "pod-1",
				Namespace: "default",
				Severity:  "Critical",
				Message:   "Running as root",
			},
		},
		ResourceIssues: []audit.ResourceIssue{
			{
				Pod:       "pod-2",
				Namespace: "default",
				Severity:  "Warning",
				Message:   "No resource limits",
			},
		},
		ProbeIssues: []audit.ProbeIssue{
			{
				Pod:       "pod-3",
				Namespace: "default",
				Severity:  "Warning",
				Message:   "Missing liveness probe",
			},
		},
		RBACIssues: []audit.RBACIssue{
			{
				Namespace: "default",
				Resource:  "Role/admin",
				Severity:  "Critical",
				Message:   "Overly permissive role",
			},
		},
		ResourceQuotaIssues: []audit.ResourceQuotaIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	err := renderAuditHTML(buf, result)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
	// Check for presence of multiple issue types
	assert.Contains(t, output, "pod-1")
	assert.Contains(t, output, "pod-2")
	assert.Contains(t, output, "admin")
}
