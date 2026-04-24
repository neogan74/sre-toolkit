package reporter

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/audit"
	"github.com/neogan/sre-toolkit/internal/k8s-doctor/diagnostics"
	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
)

// htmlSection represents a collapsible table section in the HTML report.
type htmlSection struct {
	Title   string
	Headers []string
	Rows    []htmlRow
}

// htmlRow represents a table row with optional severity-based highlighting.
type htmlRow struct {
	Severity string
	Cells    []string
}

// healthCheckViewData is the view model passed to the healthcheck HTML template.
type healthCheckViewData struct {
	GeneratedAt     string
	Nodes           []healthcheck.NodeStatus
	Pods            *healthcheck.PodStatus
	Components      []healthcheck.ComponentStatus
	NetworkPolicies *healthcheck.NetworkPoliciesStatus
	PodChartJSON    template.JS
	NodeChartJSON   template.JS
}

// issueReportViewData is the shared view model for diagnostics and audit HTML reports.
type issueReportViewData struct {
	Title       string
	GeneratedAt string
	Total       int
	Critical    int
	Warning     int
	Info        int
	ChartJSON   template.JS
	Sections    []htmlSection
}

// doughnutChart builds the Chart.js data object for a doughnut chart.
func doughnutChart(labels []string, values []int, colors []string) map[string]interface{} {
	return map[string]interface{}{
		"labels": labels,
		"datasets": []map[string]interface{}{
			{
				"data":            values,
				"backgroundColor": colors,
				"borderWidth":     0,
				"hoverOffset":     4,
			},
		},
	}
}

// severityChart builds a severity breakdown doughnut chart.
func severityChart(critical, warning, info int) map[string]interface{} {
	return doughnutChart(
		[]string{"Critical", "Warning", "Info"},
		[]int{critical, warning, info},
		[]string{"#ef4444", "#f59e0b", "#3b82f6"},
	)
}

// renderHealthCheckHTML writes a complete HTML health report to w.
func renderHealthCheckHTML(w io.Writer, nodes []healthcheck.NodeStatus, pods *healthcheck.PodStatus, components []healthcheck.ComponentStatus, netPols *healthcheck.NetworkPoliciesStatus) error {
	podChartRaw, _ := json.Marshal(doughnutChart(
		[]string{"Running", "Pending", "Failed", "Succeeded", "Unknown"},
		[]int{pods.Running, pods.Pending, pods.Failed, pods.Succeeded, pods.Unknown},
		[]string{"#22c55e", "#f59e0b", "#ef4444", "#3b82f6", "#94a3b8"},
	))

	readyCount, notReadyCount := 0, 0
	for _, n := range nodes {
		if n.Status == "Ready" {
			readyCount++
		} else {
			notReadyCount++
		}
	}
	nodeChartRaw, _ := json.Marshal(doughnutChart(
		[]string{"Ready", "Not Ready"},
		[]int{readyCount, notReadyCount},
		[]string{"#22c55e", "#ef4444"},
	))

	data := healthCheckViewData{
		GeneratedAt:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Nodes:           nodes,
		Pods:            pods,
		Components:      components,
		NetworkPolicies: netPols,
		PodChartJSON:    template.JS(podChartRaw),  //nolint:gosec
		NodeChartJSON:   template.JS(nodeChartRaw), //nolint:gosec
	}

	tmpl, err := template.New("hc").Funcs(htmlFuncMap()).Parse(healthCheckHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parse healthcheck template: %w", err)
	}
	return tmpl.Execute(w, data)
}

// renderDiagnosticsHTML writes a complete HTML diagnostics report to w.
func renderDiagnosticsHTML(w io.Writer, result *diagnostics.Result) error {
	chartRaw, _ := json.Marshal(severityChart(
		result.Summary.CriticalCount,
		result.Summary.WarningCount,
		result.Summary.InfoCount,
	))

	var sections []htmlSection

	if len(result.NodeIssues) > 0 {
		rows := make([]htmlRow, len(result.NodeIssues))
		for i, iss := range result.NodeIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Node, iss.Type, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Node Issues (%d)", len(result.NodeIssues)),
			Headers: []string{"Node", "Type", "Message"},
			Rows:    rows,
		})
	}

	if len(result.PodIssues) > 0 {
		rows := make([]htmlRow, len(result.PodIssues))
		for i, iss := range result.PodIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Pod, iss.Type, iss.Message, fmt.Sprintf("%d", iss.Restarts)}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Pod Issues (%d)", len(result.PodIssues)),
			Headers: []string{"Namespace", "Pod", "Type", "Message", "Restarts"},
			Rows:    rows,
		})
	}

	if len(result.SystemIssues) > 0 {
		rows := make([]htmlRow, len(result.SystemIssues))
		for i, iss := range result.SystemIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Component, iss.Type, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("System Issues (%d)", len(result.SystemIssues)),
			Headers: []string{"Component", "Type", "Message"},
			Rows:    rows,
		})
	}

	if len(result.EventIssues) > 0 {
		rows := make([]htmlRow, len(result.EventIssues))
		for i, iss := range result.EventIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Object, iss.Reason, fmt.Sprintf("%d", iss.Count), iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Event Issues (%d)", len(result.EventIssues)),
			Headers: []string{"Namespace", "Object", "Reason", "Count", "Message"},
			Rows:    rows,
		})
	}

	if len(result.ResourceIssues) > 0 {
		rows := make([]htmlRow, len(result.ResourceIssues))
		for i, iss := range result.ResourceIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Pod, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Resource Issues (%d)", len(result.ResourceIssues)),
			Headers: []string{"Namespace", "Pod", "Message"},
			Rows:    rows,
		})
	}

	if len(result.ProbeIssues) > 0 {
		rows := make([]htmlRow, len(result.ProbeIssues))
		for i, iss := range result.ProbeIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Pod, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Probe Issues (%d)", len(result.ProbeIssues)),
			Headers: []string{"Namespace", "Pod", "Message"},
			Rows:    rows,
		})
	}

	if len(result.SecurityIssues) > 0 {
		rows := make([]htmlRow, len(result.SecurityIssues))
		for i, iss := range result.SecurityIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Pod, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Security Issues (%d)", len(result.SecurityIssues)),
			Headers: []string{"Namespace", "Pod", "Message"},
			Rows:    rows,
		})
	}

	if len(result.NetworkPolicyIssues) > 0 {
		rows := make([]htmlRow, len(result.NetworkPolicyIssues))
		for i, iss := range result.NetworkPolicyIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Network Policy Issues (%d)", len(result.NetworkPolicyIssues)),
			Headers: []string{"Namespace", "Message"},
			Rows:    rows,
		})
	}

	data := issueReportViewData{
		Title:       "Diagnostics Report",
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Total:       result.Summary.TotalIssues,
		Critical:    result.Summary.CriticalCount,
		Warning:     result.Summary.WarningCount,
		Info:        result.Summary.InfoCount,
		ChartJSON:   template.JS(chartRaw), //nolint:gosec
		Sections:    sections,
	}

	tmpl, err := template.New("diag").Funcs(htmlFuncMap()).Parse(issueReportHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parse diagnostics template: %w", err)
	}
	return tmpl.Execute(w, data)
}

// renderAuditHTML writes a complete HTML audit report to w.
func renderAuditHTML(w io.Writer, result *audit.Result) error {
	chartRaw, _ := json.Marshal(severityChart(
		result.Summary.CriticalCount,
		result.Summary.WarningCount,
		result.Summary.InfoCount,
	))

	var sections []htmlSection

	if len(result.SecurityIssues) > 0 {
		rows := make([]htmlRow, len(result.SecurityIssues))
		for i, iss := range result.SecurityIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Pod, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Security Issues (%d)", len(result.SecurityIssues)),
			Headers: []string{"Namespace", "Pod", "Message"},
			Rows:    rows,
		})
	}

	if len(result.ResourceIssues) > 0 {
		rows := make([]htmlRow, len(result.ResourceIssues))
		for i, iss := range result.ResourceIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Pod, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Resource Issues (%d)", len(result.ResourceIssues)),
			Headers: []string{"Namespace", "Pod", "Message"},
			Rows:    rows,
		})
	}

	if len(result.ProbeIssues) > 0 {
		rows := make([]htmlRow, len(result.ProbeIssues))
		for i, iss := range result.ProbeIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Pod, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Probe Issues (%d)", len(result.ProbeIssues)),
			Headers: []string{"Namespace", "Pod", "Message"},
			Rows:    rows,
		})
	}

	if len(result.RBACIssues) > 0 {
		rows := make([]htmlRow, len(result.RBACIssues))
		for i, iss := range result.RBACIssues {
			ns := iss.Namespace
			if ns == "" {
				ns = "-"
			}
			subj := iss.Subject
			if subj == "" {
				subj = "-"
			}
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{ns, iss.Resource, subj, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("RBAC Issues (%d)", len(result.RBACIssues)),
			Headers: []string{"Namespace", "Resource", "Subject", "Message"},
			Rows:    rows,
		})
	}

	if len(result.ResourceQuotaIssues) > 0 {
		rows := make([]htmlRow, len(result.ResourceQuotaIssues))
		for i, iss := range result.ResourceQuotaIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Resource Quota Issues (%d)", len(result.ResourceQuotaIssues)),
			Headers: []string{"Namespace", "Message"},
			Rows:    rows,
		})
	}

	if len(result.NetworkPolicyIssues) > 0 {
		rows := make([]htmlRow, len(result.NetworkPolicyIssues))
		for i, iss := range result.NetworkPolicyIssues {
			rows[i] = htmlRow{Severity: iss.Severity, Cells: []string{iss.Namespace, iss.Message}}
		}
		sections = append(sections, htmlSection{
			Title:   fmt.Sprintf("Network Policy Issues (%d)", len(result.NetworkPolicyIssues)),
			Headers: []string{"Namespace", "Message"},
			Rows:    rows,
		})
	}

	data := issueReportViewData{
		Title:       "Audit Report",
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Total:       result.Summary.TotalIssues,
		Critical:    result.Summary.CriticalCount,
		Warning:     result.Summary.WarningCount,
		Info:        result.Summary.InfoCount,
		ChartJSON:   template.JS(chartRaw), //nolint:gosec
		Sections:    sections,
	}

	tmpl, err := template.New("audit").Funcs(htmlFuncMap()).Parse(issueReportHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parse audit template: %w", err)
	}
	return tmpl.Execute(w, data)
}

// htmlFuncMap returns the template function map for HTML templates.
func htmlFuncMap() template.FuncMap {
	return template.FuncMap{
		"severityClass": func(s string) string {
			switch s {
			case "Critical":
				return "badge-critical"
			case "Warning":
				return "badge-warning"
			default:
				return "badge-info"
			}
		},
		"nodeStatusClass": func(s string) string {
			if s == "Ready" {
				return "status-ok"
			}
			return "status-err"
		},
		"componentStatusClass": func(s string) string {
			if s == "Healthy" {
				return "status-ok"
			}
			return "status-err"
		},
		"joinRoles": func(roles []string) string {
			return strings.Join(roles, ", ")
		},
		"joinIssues": func(issues []string) string {
			if len(issues) == 0 {
				return "—"
			}
			return strings.Join(issues, "; ")
		},
	}
}

// ---------------------------------------------------------------------------
// HTML Templates
// ---------------------------------------------------------------------------

const htmlBaseCSS = `
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f1f5f9;color:#0f172a;line-height:1.5;font-size:14px}
.container{max-width:1280px;margin:0 auto;padding:2rem 1.5rem}
header{display:flex;justify-content:space-between;align-items:flex-end;margin-bottom:2rem;padding-bottom:1rem;border-bottom:2px solid #e2e8f0}
header h1{font-size:1.5rem;font-weight:700;display:flex;align-items:center;gap:.5rem}
.meta{color:#64748b;font-size:.8rem}
.stat-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:1rem;margin-bottom:2rem}
.stat-card{background:#fff;border:1px solid #e2e8f0;border-radius:10px;padding:1.25rem 1rem;text-align:center;box-shadow:0 1px 3px rgba(0,0,0,.07)}
.stat-card.s-critical{border-top:3px solid #ef4444}
.stat-card.s-warning{border-top:3px solid #f59e0b}
.stat-card.s-info{border-top:3px solid #3b82f6}
.stat-card.s-success{border-top:3px solid #22c55e}
.stat-card.s-neutral{border-top:3px solid #94a3b8}
.stat-value{font-size:2rem;font-weight:700;line-height:1}
.stat-card.s-critical .stat-value{color:#ef4444}
.stat-card.s-warning .stat-value{color:#d97706}
.stat-card.s-info .stat-value{color:#3b82f6}
.stat-card.s-success .stat-value{color:#16a34a}
.stat-card.s-neutral .stat-value{color:#64748b}
.stat-label{color:#64748b;font-size:.7rem;text-transform:uppercase;letter-spacing:.06em;margin-top:.35rem}
.charts-row{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:1rem;margin-bottom:2rem}
.chart-card{background:#fff;border:1px solid #e2e8f0;border-radius:10px;padding:1.25rem;box-shadow:0 1px 3px rgba(0,0,0,.07)}
.chart-card h2{font-size:.75rem;font-weight:600;text-transform:uppercase;letter-spacing:.06em;color:#64748b;margin-bottom:.75rem}
.chart-wrap{height:220px;display:flex;align-items:center;justify-content:center}
section{margin-bottom:1.5rem}
section>h2{font-size:.9rem;font-weight:600;color:#334155;margin-bottom:.75rem;padding-bottom:.4rem;border-bottom:1px solid #e2e8f0}
details{background:#fff;border:1px solid #e2e8f0;border-radius:10px;margin-bottom:.75rem;box-shadow:0 1px 3px rgba(0,0,0,.05);overflow:hidden}
details[open] summary{border-bottom:1px solid #e2e8f0}
summary{padding:.85rem 1rem;cursor:pointer;font-weight:600;font-size:.85rem;list-style:none;display:flex;justify-content:space-between;align-items:center;user-select:none}
summary::-webkit-details-marker{display:none}
summary::after{content:'▸';font-size:.8rem;color:#94a3b8;transition:transform .2s}
details[open] summary::after{transform:rotate(90deg)}
.tbl-wrap{overflow-x:auto}
table{width:100%;border-collapse:collapse}
th{background:#f8fafc;text-align:left;padding:.6rem 1rem;font-size:.7rem;font-weight:600;text-transform:uppercase;letter-spacing:.06em;color:#64748b;white-space:nowrap}
td{padding:.65rem 1rem;border-top:1px solid #f1f5f9;font-size:.82rem;vertical-align:top}
tr:hover td{background:#fafafa}
.badge{display:inline-block;padding:.15rem .55rem;border-radius:9999px;font-size:.7rem;font-weight:700;white-space:nowrap}
.badge-critical{background:#fef2f2;color:#dc2626}
.badge-warning{background:#fffbeb;color:#d97706}
.badge-info{background:#eff6ff;color:#2563eb}
.status-ok{color:#16a34a;font-weight:600}
.status-err{color:#dc2626;font-weight:600}
.empty{text-align:center;padding:2rem;color:#94a3b8;font-style:italic}
.all-good{background:#f0fdf4;border:1px solid #86efac;border-radius:10px;padding:1.25rem 1.5rem;color:#15803d;font-weight:600;text-align:center;margin-bottom:1rem}
`

const healthCheckHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>k8s-doctor — Health Report</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
<style>` + htmlBaseCSS + `</style>
</head>
<body>
<div class="container">
  <header>
    <h1>☸ Kubernetes Health Report</h1>
    <span class="meta">Generated: {{.GeneratedAt}}</span>
  </header>

  <div class="stat-grid">
    <div class="stat-card s-neutral"><div class="stat-value">{{len .Nodes}}</div><div class="stat-label">Nodes</div></div>
    <div class="stat-card s-neutral"><div class="stat-value">{{.Pods.Total}}</div><div class="stat-label">Total Pods</div></div>
    <div class="stat-card s-success"><div class="stat-value">{{.Pods.Running}}</div><div class="stat-label">Running</div></div>
    <div class="stat-card s-warning"><div class="stat-value">{{.Pods.Pending}}</div><div class="stat-label">Pending</div></div>
    <div class="stat-card s-critical"><div class="stat-value">{{.Pods.Failed}}</div><div class="stat-label">Failed</div></div>
    <div class="stat-card s-neutral"><div class="stat-value">{{len .Pods.ProblemPods}}</div><div class="stat-label">Problem Pods</div></div>
  </div>

  <div class="charts-row">
    <div class="chart-card">
      <h2>Pod Status Distribution</h2>
      <div class="chart-wrap"><canvas id="podChart"></canvas></div>
    </div>
    <div class="chart-card">
      <h2>Node Readiness</h2>
      <div class="chart-wrap"><canvas id="nodeChart"></canvas></div>
    </div>
  </div>

  <section>
    <h2>Nodes</h2>
    <div class="tbl-wrap">
      <table>
        <thead><tr><th>Name</th><th>Status</th><th>Roles</th><th>Version</th><th>CPU</th><th>Memory</th><th>Issues</th></tr></thead>
        <tbody>
        {{range .Nodes}}
          <tr>
            <td>{{.Name}}</td>
            <td class="{{nodeStatusClass .Status}}">{{.Status}}</td>
            <td>{{joinRoles .Roles}}</td>
            <td>{{.Version}}</td>
            <td>{{.CPUUsage}}</td>
            <td>{{.MemoryUsage}}</td>
            <td>{{joinIssues .Issues}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </div>
  </section>

  <section>
    <h2>Components</h2>
    <div class="tbl-wrap">
      <table>
        <thead><tr><th>Component</th><th>Status</th><th>Message</th></tr></thead>
        <tbody>
        {{range .Components}}
          <tr>
            <td>{{.Name}}</td>
            <td class="{{componentStatusClass .Status}}">{{.Status}}</td>
            <td>{{.Message}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </div>
  </section>

  {{if .Pods.ProblemPods}}
  <section>
    <h2>Problem Pods ({{len .Pods.ProblemPods}})</h2>
    <div class="tbl-wrap">
      <table>
        <thead><tr><th>Namespace</th><th>Pod</th><th>Status</th><th>Reason</th><th>Restarts</th></tr></thead>
        <tbody>
        {{range .Pods.ProblemPods}}
          <tr>
            <td>{{.Namespace}}</td>
            <td>{{.Name}}</td>
            <td>{{.Status}}</td>
            <td>{{.Reason}}</td>
            <td>{{.Restarts}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </div>
  </section>
  {{end}}

  <section>
    <h2>Network Policies</h2>
    <div class="stat-grid" style="grid-template-columns:repeat(2,minmax(120px,200px))">
      <div class="stat-card s-neutral"><div class="stat-value">{{.NetworkPolicies.TotalNamespaces}}</div><div class="stat-label">Namespaces</div></div>
      <div class="stat-card s-info"><div class="stat-value">{{.NetworkPolicies.TotalPolicies}}</div><div class="stat-label">Policies</div></div>
    </div>
    {{if .NetworkPolicies.Issues}}
    <div class="tbl-wrap">
      <table>
        <thead><tr><th>Namespace</th><th>Severity</th><th>Message</th></tr></thead>
        <tbody>
        {{range .NetworkPolicies.Issues}}
          <tr>
            <td>{{.Namespace}}</td>
            <td><span class="badge {{severityClass .Severity}}">{{.Severity}}</span></td>
            <td>{{.Message}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </div>
    {{else}}
    <p class="empty">No network policy issues found.</p>
    {{end}}
  </section>
</div>

<script>
(function(){
  const chartDefaults = {
    type: 'doughnut',
    options: {
      responsive: true,
      maintainAspectRatio: true,
      cutout: '65%',
      plugins: {
        legend: { position: 'right', labels: { boxWidth: 12, font: { size: 11 } } }
      }
    }
  };
  new Chart(document.getElementById('podChart'), Object.assign({}, chartDefaults, { data: {{.PodChartJSON}} }));
  new Chart(document.getElementById('nodeChart'), Object.assign({}, chartDefaults, { data: {{.NodeChartJSON}} }));
})();
</script>
</body>
</html>`

const issueReportHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>k8s-doctor — {{.Title}}</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
<style>` + htmlBaseCSS + `</style>
</head>
<body>
<div class="container">
  <header>
    <h1>☸ {{.Title}}</h1>
    <span class="meta">Generated: {{.GeneratedAt}}</span>
  </header>

  <div class="stat-grid">
    <div class="stat-card s-neutral"><div class="stat-value">{{.Total}}</div><div class="stat-label">Total Issues</div></div>
    <div class="stat-card s-critical"><div class="stat-value">{{.Critical}}</div><div class="stat-label">Critical</div></div>
    <div class="stat-card s-warning"><div class="stat-value">{{.Warning}}</div><div class="stat-label">Warning</div></div>
    <div class="stat-card s-info"><div class="stat-value">{{.Info}}</div><div class="stat-label">Info</div></div>
  </div>

  <div class="charts-row">
    <div class="chart-card" style="max-width:360px">
      <h2>Issues by Severity</h2>
      <div class="chart-wrap"><canvas id="sevChart"></canvas></div>
    </div>
  </div>

  {{if not .Sections}}
  <div class="all-good">✓ No issues found. Cluster looks healthy!</div>
  {{else}}
  <section>
    <h2>Issue Details</h2>
    {{range .Sections}}
    <details open>
      <summary>{{.Title}}</summary>
      <div class="tbl-wrap">
        <table>
          <thead>
            <tr>
              <th>Severity</th>
              {{range .Headers}}<th>{{.}}</th>{{end}}
            </tr>
          </thead>
          <tbody>
            {{range .Rows}}
            <tr>
              <td><span class="badge {{severityClass .Severity}}">{{.Severity}}</span></td>
              {{range .Cells}}<td>{{.}}</td>{{end}}
            </tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </details>
    {{end}}
  </section>
  {{end}}
</div>

<script>
(function(){
  new Chart(document.getElementById('sevChart'), {
    type: 'doughnut',
    data: {{.ChartJSON}},
    options: {
      responsive: true,
      maintainAspectRatio: true,
      cutout: '65%',
      plugins: {
        legend: { position: 'right', labels: { boxWidth: 12, font: { size: 11 } } }
      }
    }
  });
})();
</script>
</body>
</html>`
