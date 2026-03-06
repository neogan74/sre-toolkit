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

func TestHelmLinter_Lint(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		chartYaml      string
		hasValues      bool
		hasTemplates   bool
		expectedIssues int
		issuePatterns  []string
	}{
		{
			name: "Valid Chart",
			chartYaml: `
apiVersion: v2
name: mychart
version: 1.0.0
description: A great chart
type: application
maintainers:
  - name: me
    email: me@example.com
icon: https://example.com/icon.png
`,
			hasValues:      true,
			hasTemplates:   true,
			expectedIssues: 0,
		},
		{
			name: "Missing Required Fields",
			chartYaml: `
apiVersion: v2
# name missing
# version missing
description: Incomplete chart
`,
			hasValues:      true,
			hasTemplates:   true,
			expectedIssues: 4, // name, version, maintainers, icon
			issuePatterns:  []string{"missing 'name'", "missing 'version'", "missing 'maintainers'", "missing 'icon'"},
		},
		{
			name: "Invalid Version",
			chartYaml: `
apiVersion: v2
name: mychart
version: latest
description: Bad version
type: application
maintainers:
  - name: me
icon: https://example.com/icon.png
`,
			hasValues:      true,
			hasTemplates:   true,
			expectedIssues: 1,
			issuePatterns:  []string{"does not look like SemVer"},
		},
		{
			name: "Missing Structure",
			chartYaml: `
apiVersion: v2
name: mychart
version: 1.0.0
description: A great chart
maintainers:
  - name: me
icon: https://example.com/icon.png
`,
			hasValues:      false,
			hasTemplates:   false,
			expectedIssues: 2,
			issuePatterns:  []string{"missing 'values.yaml'", "missing 'templates/' directory"},
		},
	}

	linter := NewHelmLinter()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create chart directory
			chartDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(chartDir, 0755)
			require.NoError(t, err)

			// Create Chart.yaml
			chartPath := filepath.Join(chartDir, "Chart.yaml")
			err = os.WriteFile(chartPath, []byte(tt.chartYaml), 0644)
			require.NoError(t, err)

			// Create values.yaml if needed
			if tt.hasValues {
				err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("key: value"), 0644)
				require.NoError(t, err)
			}

			// Create templates dir if needed
			if tt.hasTemplates {
				err = os.Mkdir(filepath.Join(chartDir, "templates"), 0755)
				require.NoError(t, err)
			}

			result, err := linter.Lint(ctx, chartPath)
			require.NoError(t, err)
			require.NotNil(t, result)

			if len(result.Issues) != tt.expectedIssues {
				t.Logf("Issues found: %v", result.Issues)
			}
			assert.Equal(t, tt.expectedIssues, len(result.Issues), "Issue count mismatch")

			// Verify specific patterns
			if len(tt.issuePatterns) > 0 {
				for _, pattern := range tt.issuePatterns {
					found := false
					for _, issue := range result.Issues {
						if strings.Contains(issue.Message, pattern) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected to find issue containing '%s', but found issues: %v", pattern, result.Issues)
				}
			}
		})
	}
}
