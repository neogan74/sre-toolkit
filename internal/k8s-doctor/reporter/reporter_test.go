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

func TestNewReporter(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatJSON, buf)
	assert.NotNil(t, reporter)
	assert.Equal(t, FormatJSON, reporter.format)
	assert.Equal(t, buf, reporter.writer)
}

func TestNewReporterNilWriter(t *testing.T) {
	reporter := NewReporter(FormatTable, nil)
	assert.NotNil(t, reporter)
	// Should default to os.Stdout
	assert.NotNil(t, reporter.writer)
}

func TestReportHealthCheckJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatJSON, buf)

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

	err := reporter.ReportHealthCheck(nodes, pods, components, netPols)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "node-1")
}

func TestReportHealthCheckTable(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

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
		ProblemPods: []healthcheck.ProblemPod{},
	}

	components := []healthcheck.ComponentStatus{}
	netPols := &healthcheck.NetworkPoliciesStatus{}

	err := reporter.ReportHealthCheck(nodes, pods, components, netPols)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "node-1")
}

func TestReportHealthCheckYAML(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatYAML, buf)

	nodes := []healthcheck.NodeStatus{
		{
			Name:   "node-1",
			Status: "Ready",
		},
	}

	pods := &healthcheck.PodStatus{Total: 10}
	components := []healthcheck.ComponentStatus{}
	netPols := &healthcheck.NetworkPoliciesStatus{}

	err := reporter.ReportHealthCheck(nodes, pods, components, netPols)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestReportHealthCheckHTML(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatHTML, buf)

	nodes := []healthcheck.NodeStatus{
		{Name: "node-1", Status: "Ready"},
	}
	pods := &healthcheck.PodStatus{Total: 10}
	components := []healthcheck.ComponentStatus{}
	netPols := &healthcheck.NetworkPoliciesStatus{}

	err := reporter.ReportHealthCheck(nodes, pods, components, netPols)
	require.NoError(t, err)
	output := buf.String()
	assert.NotEmpty(t, output)
	// HTML should contain html tag or DOCTYPE
	assert.True(t, bytes.Contains(buf.Bytes(), []byte("<")) || bytes.Contains(buf.Bytes(), []byte("html")))
}

func TestReportNodeHealth(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatJSON, buf)

	nodes := []healthcheck.NodeStatus{
		{Name: "node-1", Status: "Ready"},
		{Name: "node-2", Status: "NotReady"},
	}

	err := reporter.ReportNodeHealth(nodes)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "node-1")
	assert.Contains(t, buf.String(), "node-2")
}

func TestReportPodHealth(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

	pods := &healthcheck.PodStatus{
		Total:     10,
		Running:   8,
		Pending:   2,
		Failed:    0,
		Succeeded: 0,
		Unknown:   0,
	}

	err := reporter.ReportPodHealth(pods)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Pod Summary")
}

func TestReportComponentHealth(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

	components := []healthcheck.ComponentStatus{
		{Name: "etcd", Status: "Healthy", Message: "OK"},
		{Name: "api-server", Status: "Warning", Message: "Slow"},
	}

	err := reporter.ReportComponentHealth(components)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "etcd")
	assert.Contains(t, buf.String(), "api-server")
}

func TestReportNetworkPolicies(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

	netPols := &healthcheck.NetworkPoliciesStatus{
		TotalNamespaces: 5,
		TotalPolicies:   10,
		Issues: []healthcheck.NetworkPolicyIssue{
			{
				Namespace: "default",
				Severity:  "Warning",
				Message:   "Missing network policy",
			},
		},
	}

	err := reporter.ReportNetworkPolicies(netPols)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Network Policies")
}

func TestReportDiagnosticsJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatJSON, buf)

	result := &diagnostics.Result{
		Summary: diagnostics.Summary{
			TotalIssues:   2,
			CriticalCount: 1,
			WarningCount:  1,
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
	}

	err := reporter.ReportDiagnostics(result)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "NodeNotReady")
}

func TestReportDiagnosticsTable(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

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
	}

	err := reporter.ReportDiagnostics(result)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Diagnostics Summary")
}

func TestReportDiagnosticsHTML(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatHTML, buf)

	result := &diagnostics.Result{
		Summary: diagnostics.Summary{
			TotalIssues: 0,
		},
	}

	err := reporter.ReportDiagnostics(result)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestReportDiagnosticsMultipleIssues(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

	result := &diagnostics.Result{
		Summary: diagnostics.Summary{
			TotalIssues:   3,
			CriticalCount: 1,
			WarningCount:  2,
			InfoCount:     0,
		},
		NodeIssues: []diagnostics.NodeIssue{
			{Node: "node-1", Severity: "Critical", Type: "NodeNotReady", Message: "Not ready"},
		},
		PodIssues: []diagnostics.PodIssue{
			{Pod: "pod-1", Namespace: "default", Severity: "Warning", Type: "CrashLoopBackOff", Restarts: 5},
		},
		SystemIssues: []diagnostics.SystemIssue{
			{Component: "etcd", Severity: "Warning", Type: "ComponentUnhealthy", Message: "Unhealthy"},
		},
	}

	err := reporter.ReportDiagnostics(result)
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Node Issues")
	assert.Contains(t, output, "Pod Issues")
	assert.Contains(t, output, "System Issues")
}

func TestReportAuditTable(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

	result := &audit.Result{
		Summary: audit.Summary{
			TotalIssues:   2,
			CriticalCount: 1,
			WarningCount:  1,
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
		RBACIssues: []audit.RBACIssue{
			{
				Namespace: "default",
				Resource:  "Role/admin",
				Severity:  "Warning",
				Message:   "Overly permissive",
			},
		},
	}

	err := reporter.ReportAudit(result)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Audit Summary")
	assert.Contains(t, buf.String(), "Security Issues")
	assert.Contains(t, buf.String(), "RBAC Issues")
}

func TestReportAuditJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatJSON, buf)

	result := &audit.Result{
		Summary: audit.Summary{
			TotalIssues: 0,
		},
	}

	err := reporter.ReportAudit(result)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestReportAuditHTML(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatHTML, buf)

	result := &audit.Result{
		Summary: audit.Summary{
			TotalIssues: 0,
		},
	}

	err := reporter.ReportAudit(result)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestFormatSeverityEmoji(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		expected string
	}{
		{"Critical", "Critical", "🔴"},
		{"Warning", "Warning", "⚠️"},
		{"Info", "Info", "ℹ️"},
		{"Unknown", "Unknown", "ℹ️"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSeverityEmoji(tt.severity)
			assert.Contains(t, result, tt.expected)
			assert.Contains(t, result, tt.severity)
		})
	}
}

func TestReportUnsupportedFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(OutputFormat("invalid"), buf)

	nodes := []healthcheck.NodeStatus{}
	err := reporter.ReportNodeHealth(nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestReportNodeTableWithIssues(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

	nodes := []healthcheck.NodeStatus{
		{
			Name:        "node-1",
			Status:      "NotReady",
			Roles:       []string{"control-plane", "worker"},
			Version:     "v1.25.0",
			CPUUsage:    "95%",
			MemoryUsage: "92%",
			Issues:      []string{"Memory pressure detected", "High CPU usage"},
		},
	}

	err := reporter.ReportNodeHealth(nodes)
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "node-1")
	assert.Contains(t, output, "NotReady")
	assert.Contains(t, output, "2")
}

func TestReportPodTableWithProblems(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

	pods := &healthcheck.PodStatus{
		Total:     10,
		Running:   7,
		Pending:   1,
		Failed:    1,
		Succeeded: 1,
		Unknown:   0,
		ProblemPods: []healthcheck.ProblemPod{
			{
				Name:      "crash-pod",
				Namespace: "default",
				Status:    "CrashLoopBackOff",
				Reason:    "CrashLoopBackOff",
				Message:   "Application crashed",
				Restarts:  15,
			},
		},
	}

	err := reporter.ReportPodHealth(pods)
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Pod Summary")
	assert.Contains(t, output, "crash-pod")
	assert.Contains(t, output, "15")
}

func TestReportEmptyDiagnostics(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

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

	err := reporter.ReportDiagnostics(result)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No issues found")
}

func TestReportEmptyAudit(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewReporter(FormatTable, buf)

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

	err := reporter.ReportAudit(result)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No audit issues found")
}

func TestReportYAMLFormats(t *testing.T) {
	tests := []struct {
		name string
		test func(*testing.T)
	}{
		{
			name: "Node health YAML",
			test: func(t *testing.T) {
				buf := &bytes.Buffer{}
				reporter := NewReporter(FormatYAML, buf)
				nodes := []healthcheck.NodeStatus{{Name: "node-1", Status: "Ready"}}
				err := reporter.ReportNodeHealth(nodes)
				require.NoError(t, err)
				assert.Contains(t, buf.String(), "name:")
			},
		},
		{
			name: "Pod health YAML",
			test: func(t *testing.T) {
				buf := &bytes.Buffer{}
				reporter := NewReporter(FormatYAML, buf)
				pods := &healthcheck.PodStatus{Total: 10}
				err := reporter.ReportPodHealth(pods)
				require.NoError(t, err)
				assert.NotEmpty(t, buf.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}
