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

func TestHelmLinter_Dependencies(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		chartYaml      string
		expectedIssues int
		issuePatterns  []string
	}{
		{
			name: "Valid Dependencies",
			chartYaml: `
apiVersion: v2
name: mychart
version: 1.0.0
description: A test chart
maintainers:
  - name: test
icon: https://example.com/icon.png
dependencies:
  - name: redis
    repository: https://charts.bitnami.com/bitnami
    version: ^17.0.0
`,
			expectedIssues: 0,
		},
		{
			name: "Missing Dependency Fields",
			chartYaml: `
apiVersion: v2
name: mychart
version: 1.0.0
description: A test chart
maintainers:
  - name: test
icon: https://example.com/icon.png
dependencies:
  - repository: https://example.com/charts
    version: 1.0.0
`,
			expectedIssues: 2,
			issuePatterns:  []string{"missing 'name' field", "version constraints"},
		},
		{
			name: "Insecure HTTP Repository",
			chartYaml: `
apiVersion: v2
name: mychart
version: 1.0.0
description: A test chart
maintainers:
  - name: test
icon: https://example.com/icon.png
dependencies:
  - name: myapp
    repository: http://example.com/charts
    version: 1.0.0
`,
			expectedIssues: 2,
			issuePatterns:  []string{"insecure HTTP repository", "version constraints"},
		},
	}

	linter := NewHelmLinter()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(chartDir, 0755)
			require.NoError(t, err)

			chartPath := filepath.Join(chartDir, "Chart.yaml")
			err = os.WriteFile(chartPath, []byte(tt.chartYaml), 0644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("key: value"), 0644)
			require.NoError(t, err)

			err = os.Mkdir(filepath.Join(chartDir, "templates"), 0755)
			require.NoError(t, err)

			result, err := linter.Lint(ctx, chartPath)
			require.NoError(t, err)
			require.NotNil(t, result)

			if len(result.Issues) != tt.expectedIssues {
				t.Logf("Issues found: %v", result.Issues)
			}
			assert.Equal(t, tt.expectedIssues, len(result.Issues), "Issue count mismatch")

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

func TestHelmLinter_KubeVersion(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		chartYaml      string
		expectedIssues int
		issuePatterns  []string
	}{
		{
			name: "Valid KubeVersion",
			chartYaml: `
apiVersion: v2
name: mychart
version: 1.0.0
description: A test chart
maintainers:
  - name: test
icon: https://example.com/icon.png
kubeVersion: ">=1.22.0-0"
`,
			expectedIssues: 0,
		},
		{
			name: "Old KubeVersion",
			chartYaml: `
apiVersion: v2
name: mychart
version: 1.0.0
description: A test chart
maintainers:
  - name: test
icon: https://example.com/icon.png
kubeVersion: ">=1.18.0-0"
`,
			expectedIssues: 1,
			issuePatterns:  []string{"end-of-life Kubernetes version"},
		},
	}

	linter := NewHelmLinter()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(chartDir, 0755)
			require.NoError(t, err)

			chartPath := filepath.Join(chartDir, "Chart.yaml")
			err = os.WriteFile(chartPath, []byte(tt.chartYaml), 0644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("key: value"), 0644)
			require.NoError(t, err)

			err = os.Mkdir(filepath.Join(chartDir, "templates"), 0755)
			require.NoError(t, err)

			result, err := linter.Lint(ctx, chartPath)
			require.NoError(t, err)
			require.NotNil(t, result)

			if len(result.Issues) != tt.expectedIssues {
				t.Logf("Issues found: %v", result.Issues)
			}
			assert.Equal(t, tt.expectedIssues, len(result.Issues), "Issue count mismatch")

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

func TestHelmLinter_Templates(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		templateYaml   string
		expectedIssues int
		issuePatterns  []string
	}{
		{
			name: "Valid Template",
			templateYaml: `apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: myapp
  name: {{ .Values.name }}
data:
  config.yaml: |
    key: {{ .Values.config.key }}
`,
			expectedIssues: 1, // template syntax error: nil Values context during static lint
			issuePatterns:  []string{"Template syntax error"},
		},
		{
			name: "Unclosed Braces",
			templateYaml: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: {{ .Values.someKey `,
			expectedIssues: 4,
			issuePatterns:  []string{"Unclosed template braces"},
		},
	}

	linter := NewHelmLinter()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(chartDir, 0755)
			require.NoError(t, err)

			chartPath := filepath.Join(chartDir, "Chart.yaml")
			chartYaml := "apiVersion: v2\nname: mychart\nversion: 1.0.0\ndescription: A test chart\nmaintainers:\n  - name: test\nicon: https://example.com/icon.png"
			err = os.WriteFile(chartPath, []byte(chartYaml), 0644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("config:\n  key: value"), 0644)
			require.NoError(t, err)

			templatesDir := filepath.Join(chartDir, "templates")
			err = os.Mkdir(templatesDir, 0755)
			require.NoError(t, err)

			templatePath := filepath.Join(templatesDir, "configmap.yaml")
			err = os.WriteFile(templatePath, []byte(tt.templateYaml), 0644)
			require.NoError(t, err)

			result, err := linter.Lint(ctx, chartPath)
			require.NoError(t, err)
			require.NotNil(t, result)

			if len(result.Issues) != tt.expectedIssues {
				t.Logf("Issues found: %v", result.Issues)
			}
			assert.Equal(t, tt.expectedIssues, len(result.Issues), "Issue count mismatch")

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

func TestHelmLinter_Values(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		valuesYaml     string
		expectedIssues int
		issuePatterns  []string
	}{
		{
			name: "Valid Values",
			valuesYaml: `
replicaCount: 1
image:
  repository: nginx
  tag: "1.21.0"
  pullPolicy: IfNotPresent
`,
			expectedIssues: 0,
		},
		{
			name: "Latest Tag",
			valuesYaml: `
image:
  repository: nginx
  tag: "nginx:latest"
`,
			expectedIssues: 1,
			issuePatterns:  []string{"'latest' image tag"},
		},
		{
			name: "Plaintext Password",
			valuesYaml: `
database:
  host: 10.0.0.1
  port: 5432
  user: admin
  password: secret123
`,
			expectedIssues: 1,
			issuePatterns:  []string{"plaintext password/secret"},
		},
	}

	linter := NewHelmLinter()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(chartDir, 0755)
			require.NoError(t, err)

			chartPath := filepath.Join(chartDir, "Chart.yaml")
			chartYaml := "apiVersion: v2\nname: mychart\nversion: 1.0.0\ndescription: A test chart\nmaintainers:\n  - name: test\nicon: https://example.com/icon.png"
			err = os.WriteFile(chartPath, []byte(chartYaml), 0644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(tt.valuesYaml), 0644)
			require.NoError(t, err)

			templatesDir := filepath.Join(chartDir, "templates")
			err = os.Mkdir(templatesDir, 0755)
			require.NoError(t, err)

			result, err := linter.Lint(ctx, chartPath)
			require.NoError(t, err)
			require.NotNil(t, result)

			if len(result.Issues) != tt.expectedIssues {
				t.Logf("Issues found: %v", result.Issues)
			}
			assert.Equal(t, tt.expectedIssues, len(result.Issues), "Issue count mismatch")

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
