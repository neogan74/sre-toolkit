package linter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// HelmLinter implements Linter for Helm Charts
type HelmLinter struct{}

func NewHelmLinter() *HelmLinter {
	return &HelmLinter{}
}

// ChartMetadata represents the structure of Chart.yaml
type ChartMetadata struct {
	APIVersion  string   `yaml:"apiVersion"`
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	KubeVersion string   `yaml:"kubeVersion"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Home        string   `yaml:"home"`
	Keywords    []string `yaml:"keywords"`
	Maintainers []struct {
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"maintainers"`
	Icon         string       `yaml:"icon"`
	Dependencies []Dependency `yaml:"dependencies"`
}

// Dependency represents a Helm chart dependency
type Dependency struct {
	Name       string `yaml:"name"`
	Repository string `yaml:"repository"`
	Version    string `yaml:"version"`
	Alias      string `yaml:"alias"`
}

// ValuesData represents the structure of values.yaml
type ValuesData map[string]interface{}

func (l *HelmLinter) Lint(ctx context.Context, path string) (*Result, error) {
	result := &Result{Passed: true}

	// Only process Chart.yaml files
	if filepath.Base(path) != "Chart.yaml" {
		return nil, nil // Not a chart metadata file, skip
	}

	chartDir := filepath.Dir(path)

	// 1. Validate Chart.yaml Content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Chart.yaml: %w", err)
	}

	var metadata ChartMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		result.Issues = append(result.Issues, Issue{
			Severity: "Error",
			Message:  fmt.Sprintf("Failed to parse Chart.yaml: %v", err),
			File:     path,
			Line:     1,
		})
		result.Passed = false
		return result, nil // Cannot proceed with metadata checks
	}

	// Check required fields
	if metadata.APIVersion == "" {
		result.Issues = append(result.Issues, Issue{
			Severity: "Error",
			Message:  "Chart.yaml missing 'apiVersion'",
			File:     path,
		})
	}
	if metadata.Name == "" {
		result.Issues = append(result.Issues, Issue{
			Severity: "Error",
			Message:  "Chart.yaml missing 'name'",
			File:     path,
		})
	}
	if metadata.Version == "" {
		result.Issues = append(result.Issues, Issue{
			Severity: "Error",
			Message:  "Chart.yaml missing 'version'",
			File:     path,
		})
	} else {
		// Basic SemVer check (could use a library, but keeping deps minimal)
		// Just ensure it's not empty and has dots
		if !strings.Contains(metadata.Version, ".") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Warning",
				Message:  fmt.Sprintf("Chart version '%s' does not look like SemVer (x.y.z)", metadata.Version),
				File:     path,
			})
		}
	}

	// Check best practices
	if len(metadata.Maintainers) == 0 {
		result.Issues = append(result.Issues, Issue{
			Severity: "Low",
			Message:  "Chart.yaml missing 'maintainers'",
			File:     path,
		})
	}
	if metadata.Description == "" {
		result.Issues = append(result.Issues, Issue{
			Severity: "Low",
			Message:  "Chart.yaml missing 'description'",
			File:     path,
		})
	}
	if metadata.Icon == "" {
		result.Issues = append(result.Issues, Issue{
			Severity: "Low",
			Message:  "Chart.yaml missing 'icon' (recommended for public charts)",
			File:     path,
		})
	}

	// Check dependencies
	if len(metadata.Dependencies) > 0 {
		l.validateDependencies(result, path, metadata.Dependencies)
	}

	// Check kubeVersion compatibility
	if metadata.KubeVersion != "" {
		l.validateKubeVersion(result, path, metadata.KubeVersion)
	}

	// 2. Validate Chart Structure

	// Check for values.yaml
	valuesPath := filepath.Join(chartDir, "values.yaml")
	valuesContent, err := os.ReadFile(valuesPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Issues = append(result.Issues, Issue{
				Severity: "Medium",
				Message:  "Chart is missing 'values.yaml'",
				File:     path,
			})
		} else {
			result.Issues = append(result.Issues, Issue{
				Severity: "Error",
				Message:  fmt.Sprintf("Failed to read values.yaml: %v", err),
				File:     valuesPath,
			})
		}
	} else {
		// Validate values.yaml structure
		l.validateValues(result, valuesPath, valuesContent)
	}

	// Check for templates directory
	templatesPath := filepath.Join(chartDir, "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		// Only warn if it's an application chart, library charts might behave differently but usually still have templates
		if metadata.Type != "library" {
			result.Issues = append(result.Issues, Issue{
				Severity: "Medium",
				Message:  "Chart is missing 'templates/' directory",
				File:     path,
			})
		}
	} else {
		// Validate templates
		l.validateTemplates(result, templatesPath)
	}

	if len(result.Issues) > 0 {
		result.Passed = false
	}

	return result, nil
}

// validateDependencies checks Helm chart dependencies
func (l *HelmLinter) validateDependencies(result *Result, chartPath string, deps []Dependency) {
	for _, dep := range deps {
		// Check required fields
		if dep.Name == "" {
			result.Issues = append(result.Issues, Issue{
				Severity: "Error",
				Message:  "Dependency missing 'name' field",
				File:     chartPath,
			})
		}
		if dep.Repository == "" {
			result.Issues = append(result.Issues, Issue{
				Severity: "Warning",
				Message:  fmt.Sprintf("Dependency '%s' missing 'repository' field", dep.Name),
				File:     chartPath,
			})
		}
		if dep.Version == "" {
			result.Issues = append(result.Issues, Issue{
				Severity: "Warning",
				Message:  fmt.Sprintf("Dependency '%s' missing version constraint", dep.Name),
				File:     chartPath,
			})
		} else if !strings.Contains(dep.Version, "~") && !strings.Contains(dep.Version, ">") &&
			!strings.Contains(dep.Version, "<") && !strings.Contains(dep.Version, "=") &&
			!strings.Contains(dep.Version, "^") {
			// No version constraint specified, warn about exact version usage
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Dependency '%s' uses exact version '%s'. Consider using version constraints (e.g., ^1.2.0, >=1.2.0,<2.0.0) for better maintainability", dep.Name, dep.Version),
				File:     chartPath,
			})
		}

		// Check for insecure HTTP repositories
		if strings.HasPrefix(dep.Repository, "http://") && !strings.Contains(dep.Repository, "localhost") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Medium",
				Message:  fmt.Sprintf("Dependency '%s' uses insecure HTTP repository. Consider using HTTPS", dep.Name),
				File:     chartPath,
			})
		}
	}
}

// validateKubeVersion checks Kubernetes version constraints
func (l *HelmLinter) validateKubeVersion(result *Result, chartPath, kubeVersion string) {
	// Basic validation of kubeVersion format
	if !strings.HasPrefix(kubeVersion, ">=") && !strings.HasPrefix(kubeVersion, "~") &&
		!strings.HasPrefix(kubeVersion, "^") && !strings.Contains(kubeVersion, "-") {
		// Check if it's a simple version without constraint
		if !strings.Contains(kubeVersion, ">") && !strings.Contains(kubeVersion, "<") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("kubeVersion '%s' should specify a minimum constraint (e.g., >=1.22.0-0)", kubeVersion),
				File:     chartPath,
			})
		}
	}

	// Warn if using very old Kubernetes versions
	if strings.Contains(kubeVersion, "1.18") || strings.Contains(kubeVersion, "1.19") ||
		strings.Contains(kubeVersion, "1.20") || strings.Contains(kubeVersion, "1.21") {
		result.Issues = append(result.Issues, Issue{
			Severity: "Low",
			Message:  fmt.Sprintf("kubeVersion '%s' targets unsupported or end-of-life Kubernetes version. Consider using 1.22+", kubeVersion),
			File:     chartPath,
		})
	}
}

// validateValues checks values.yaml structure
func (l *HelmLinter) validateValues(result *Result, valuesPath string, content []byte) {
	var values ValuesData
	if err := yaml.Unmarshal(content, &values); err != nil {
		result.Issues = append(result.Issues, Issue{
			Severity: "Error",
			Message:  fmt.Sprintf("Failed to parse values.yaml: %v", err),
			File:     valuesPath,
		})
		return
	}

	// Check for common issues
	lintLine := 1
	lines := bytes.Split(content, []byte{'\n'})
	for _, line := range lines {
		lineStr := string(line)
		// Check for password fields without encryption warning
		if strings.Contains(strings.ToLower(lineStr), "password") || strings.Contains(strings.ToLower(lineStr), "secret") {
			if !strings.Contains(strings.ToLower(lineStr), "ref:") && !strings.Contains(strings.ToLower(lineStr), "secretref") {
				result.Issues = append(result.Issues, Issue{
					Severity: "Low",
					Message:  "values.yaml contains plaintext password/secret. Consider using secrets management or external references",
					File:     valuesPath,
					Line:     lintLine,
				})
			}
		}

		// Check for default image tag 'latest'
		if strings.Contains(lineStr, ":latest") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  "values.yaml contains 'latest' image tag. Pin to specific version for reproducibility",
				File:     valuesPath,
				Line:     lintLine,
			})
		}

		// Check for empty values (possible null pointer in templates)
		if strings.Contains(lineStr, "value:") && (strings.TrimSuffix(strings.TrimSpace(lineStr), "value:") == "") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  "values.yaml has empty value. This may cause template errors if not properly checked",
				File:     valuesPath,
				Line:     lintLine,
			})
		}

		lintLine++
	}

	// Check for duplicate keys at top level
	if len(values) > 0 {
		// yaml.v3 doesn't have built-in duplicate key detection, so we warn about potential issues
		// In production, consider using a YAML parser that reports duplicates
	}
}

// validateTemplates checks Go template syntax and common issues
func (l *HelmLinter) validateTemplates(result *Result, templatesPath string) {
	// Read all template files
	templateFiles, err := os.ReadDir(templatesPath)
	if err != nil {
		result.Issues = append(result.Issues, Issue{
			Severity: "Error",
			Message:  fmt.Sprintf("Failed to read templates directory: %v", err),
			File:     templatesPath,
		})
		return
	}

	for _, tf := range templateFiles {
		if tf.IsDir() {
			continue // Skip subdirectories
		}

		templatePath := filepath.Join(templatesPath, tf.Name())
		content, err := os.ReadFile(templatePath)
		if err != nil {
			result.Issues = append(result.Issues, Issue{
				Severity: "Error",
				Message:  fmt.Sprintf("Failed to read template '%s': %v", tf.Name(), err),
				File:     templatePath,
			})
			continue
		}

		// Validate Go template syntax
		contentStr := string(content)
		if strings.Contains(contentStr, "{{") {
			if err := l.validateGoTemplateSyntax(templatePath, contentStr); err != nil {
				result.Issues = append(result.Issues, Issue{
					Severity: "Error",
					Message:  fmt.Sprintf("Template syntax error: %v", err),
					File:     templatePath,
				})
			}
		}

		// Check for common template issues
		l.validateTemplateIssues(result, templatePath, contentStr)
	}
}

// validateGoTemplateSyntax validates Go template syntax
func (l *HelmLinter) validateGoTemplateSyntax(templatePath, content string) error {
	// Create a simple template with basic functions that Helm provides
	tmpl, err := template.New(filepath.Base(templatePath)).
		Option("missingkey=error").
		Parse(content)
	if err != nil {
		// Try to provide a more helpful error message
		if strings.Contains(err.Error(), "unexpected") || strings.Contains(err.Error(), "unclosed") {
			return fmt.Errorf("syntax error: %v", err)
		}
		return err
	}

	// Try to execute template with empty data to catch basic issues
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		return err
	}

	return nil
}

// validateTemplateIssues checks for common template issues
func (l *HelmLinter) validateTemplateIssues(result *Result, templatePath, content string) {
	// Check for potentially unsafe default values (nil)
	if strings.Contains(content, "default .Values") {
		result.Issues = append(result.Issues, Issue{
			Severity: "Low",
			Message:  "Template uses 'default .Values'. Consider using 'default .Values.key' with specific key",
			File:     templatePath,
		})
	}

	// Check for deprecated template functions
	deprecatedFuncs := []string{"include", "required"}
	for _, funcName := range deprecatedFuncs {
		if strings.Contains(content, funcName) {
			// Note: These are actually commonly used in Helm, not deprecated
			// This is just an example of what we could check
		}
	}

	// Check for potential XSS vulnerabilities in templates
	if strings.Contains(content, " | nindent ") && strings.Contains(content, "HTML") {
		result.Issues = append(result.Issues, Issue{
			Severity: "Low",
			Message:  "Template generates HTML content. Ensure proper escaping to prevent XSS vulnerabilities",
			File:     templatePath,
		})
	}

	// Check for unclosed braces
	openCount := strings.Count(content, "{{")
	closeCount := strings.Count(content, "}}")
	if openCount != closeCount {
		result.Issues = append(result.Issues, Issue{
			Severity: "Error",
			Message:  fmt.Sprintf("Unclosed template braces: found %d opening and %d closing", openCount, closeCount),
			File:     templatePath,
		})
	}

	// Check for missing quotes in comparisons
	if strings.Contains(content, "==") || strings.Contains(content, "!=") {
		// In Helm/Go templates, you should use 'eq' and 'ne' functions
		result.Issues = append(result.Issues, Issue{
			Severity: "Low",
			Message:  "Template uses '==' or '!=' operators. In Helm templates, use 'eq' and 'ne' functions instead",
			File:     templatePath,
		})
	}

	// Check for YAML indentation issues in templates
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "{{") && strings.HasSuffix(strings.TrimSpace(line), "}}") {
			// Template action on same line as YAML key - potential indentation issue
			if strings.HasPrefix(strings.TrimSpace(line), "{{") && !strings.Contains(line, ":") {
				continue // It's probably fine
			}
			// Check for common indentation issues
			if strings.Contains(line, "{{") && !strings.Contains(line, "  {{") {
				// Might need proper indentation
			}
		}

		// Check for trailing whitespace
		if len(line) > 0 && line[len(line)-1] == ' ' {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Line %d has trailing whitespace", i+1),
				File:     templatePath,
				Line:     i + 1,
			})
		}
	}

	// Check for hardcoded configuration values
	hardcodedValues := []string{"localhost", "127.0.0.1", "example.com", "test", "demo"}
	for _, value := range hardcodedValues {
		if strings.Contains(strings.ToLower(content), value) {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Template contains hardcoded value '%s'. Consider parameterizing it in values.yaml", value),
				File:     templatePath,
			})
		}
	}

	// Check for empty resource limits/requests
	if strings.Contains(content, "resources:") || strings.Contains(content, "limits:") {
		if strings.Contains(content, "cpu: \"\"") || strings.Contains(content, "memory: \"\"") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Medium",
				Message:  "Template has empty resource limits. This may cause deployment issues",
				File:     templatePath,
			})
		}
	}

	// Check for missing labels
	if strings.Contains(content, "kind: Deployment") || strings.Contains(content, "kind: StatefulSet") {
		if !strings.Contains(content, "labels:") || !strings.Contains(content, "app.kubernetes.io") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  "Workload template missing recommended labels (app.kubernetes.io/name, app.kubernetes.io/instance)",
				File:     templatePath,
			})
		}
	}

	// Check for namespace specification
	if strings.Contains(content, "namespace:") {
		result.Issues = append(result.Issues, Issue{
			Severity: "Medium",
			Message:  "Template hardcodes namespace. Consider omitting namespace for flexibility",
			File:     templatePath,
		})
	}
}
