package linter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetesLinter_Lint(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		content        string
		expectedPassed bool
		minIssues      int // Use minimum issues count since we added more checks
		issueContains  []string
	}{
		{
			name: "Fully Compliant Pod",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: compliant-pod
spec:
  containers:
  - name: compliant
    image: nginx:1.19
    securityContext:
      runAsNonRoot: true
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
`,
			expectedPassed: true,
			minIssues:      0,
		},
		{
			name: "No Limits",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: no-limits
spec:
  containers:
  - name: no-limits
    image: nginx:1.19
`,
			expectedPassed: false,
			minIssues:      1, // At least "no resource limits"
			issueContains:  []string{"no resource limits"},
		},
		{
			name: "Privileged Container",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: privileged
spec:
  containers:
  - name: privileged
    image: nginx:1.19
    securityContext:
      privileged: true
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
`,
			expectedPassed: false,
			minIssues:      1,
			issueContains:  []string{"is privileged"},
		},
		{
			name: "Latest Tag",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: latest-tag
spec:
  containers:
  - name: latest
    image: nginx:latest
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
`,
			expectedPassed: false,
			minIssues:      1,
			issueContains:  []string{"uses 'latest' tag"},
		},
		{
			name: "Host Network",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: host-network
spec:
  hostNetwork: true
  containers:
  - name: host
    image: nginx:1.19
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
`,
			expectedPassed: false,
			minIssues:      1,
			issueContains:  []string{"uses hostNetwork: true"},
		},
		{
			name: "Dangerous Capability SYS_ADMIN",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: dangerous-cap
spec:
  containers:
  - name: cap-container
    image: nginx:1.19
    securityContext:
      capabilities:
        add:
        - SYS_ADMIN
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
`,
			expectedPassed: false,
			minIssues:      1,
			issueContains:  []string{"dangerous capability: SYS_ADMIN"},
		},
		{
			name: "Missing Probes",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: no-probes
spec:
  containers:
  - name: no-probes
    image: nginx:1.19
    securityContext:
      runAsNonRoot: true
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
`,
			expectedPassed: false,
			minIssues:      2, // Missing liveness and readiness probes
			issueContains:  []string{"no livenessProbe", "no readinessProbe"},
		},
		{
			name: "Allow Privilege Escalation",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: priv-escalation
spec:
  containers:
  - name: escalation
    image: nginx:1.19
    securityContext:
      allowPrivilegeEscalation: true
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
`,
			expectedPassed: false,
			minIssues:      1,
			issueContains:  []string{"allows privilege escalation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".yaml")
			err := os.WriteFile(path, []byte(tt.content), 0644)
			require.NoError(t, err)

			linter := NewKubernetesLinter()
			result, err := linter.Lint(context.Background(), path)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedPassed, result.Passed, "Expected passed=%v but got passed=%v with issues: %v", tt.expectedPassed, result.Passed, result.Issues)
			assert.GreaterOrEqual(t, len(result.Issues), tt.minIssues, "Expected at least %d issues but got %d: %v", tt.minIssues, len(result.Issues), result.Issues)

			if len(tt.issueContains) > 0 {
				for _, substr := range tt.issueContains {
					found := false
					for _, issue := range result.Issues {
						if contains(issue.Message, substr) { // strings.Contains
							found = true
							break
						}
					}
					assert.True(t, found, "Expected issue message containing '%s', but got: %v", substr, result.Issues)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
