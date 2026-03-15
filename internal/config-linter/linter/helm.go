package linter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Maintainers []struct {
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"maintainers"`
	Icon string `yaml:"icon"`
}

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

	// 2. Validate Chart Structure

	// Check for values.yaml
	valuesPath := filepath.Join(chartDir, "values.yaml")
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		result.Issues = append(result.Issues, Issue{
			Severity: "Medium",
			Message:  "Chart is missing 'values.yaml'",
			File:     path, // blameworthy file is the Chart definition
		})
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
	}

	if len(result.Issues) > 0 {
		result.Passed = false
	}

	return result, nil
}
