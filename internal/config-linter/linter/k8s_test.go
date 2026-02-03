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
		expectedIssues int
		issueContains  []string
	}{
		{
			name: "Valid Pod",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: valid-pod
spec:
  containers:
  - name: valid
    image: nginx:1.19
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
`,
			expectedPassed: true,
			expectedIssues: 0,
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
			expectedIssues: 1, // No CPU limit, No Mem limit (actually 1 "no resource limits" if limits is nil)
			// Wait, my logic: if Limits == nil -> 1 issue.
			issueContains: []string{"no resource limits"},
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
			expectedIssues: 1,
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
			expectedPassed: false, // passed is false if len(issues) > 0. Tag check is Low severity.
			expectedIssues: 1,
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
			expectedIssues: 1,
			issueContains:  []string{"uses hostNetwork: true"},
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

			assert.Equal(t, tt.expectedPassed, result.Passed)
			assert.Len(t, result.Issues, tt.expectedIssues)

			if len(tt.issueContains) > 0 {
				for _, substr := range tt.issueContains {
					found := false
					for _, issue := range result.Issues {
						if contains(issue.Message, substr) { // strings.Contains
							found = true
							break
						}
					}
					assert.True(t, found, "Expected issue message passing '%s', but got: %v", substr, result.Issues)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
