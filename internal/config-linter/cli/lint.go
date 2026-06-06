package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neogan/sre-toolkit/internal/config-linter/linter"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/spf13/cobra"
)

// severityRank maps severity strings to a numeric rank for comparison.
var severityRank = map[string]int{
	"Low":      1,
	"Medium":   2,
	"High":     3,
	"Critical": 4,
	"Error":    3,
}

func newLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "lint",
		Short:        "Lint configuration files",
		Long:         `Scan directories or files for configuration issues in Kubernetes YAML, Helm charts, Dockerfiles, and Terraform.`,
		RunE:         runLint,
		SilenceUsage: true,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to directory or file to lint")
	cmd.Flags().StringP("output", "o", "table", "Output format (table, json)")
	cmd.Flags().StringP("min-severity", "s", "Low", "Minimum severity to report (Low, Medium, High, Critical)")
	return cmd
}

// LintReport is the JSON output structure
type LintReport struct {
	Path        string         `json:"path"`
	FilesPassed int            `json:"files_passed"`
	FilesFailed int            `json:"files_failed"`
	TotalIssues int            `json:"total_issues"`
	Issues      []linter.Issue `json:"issues,omitempty"`
}

func runLint(cmd *cobra.Command, args []string) error { //nolint:gocyclo // complex lint runner with many file type branches
	path, err := cmd.Flags().GetString("path")
	if err != nil {
		return fmt.Errorf("failed to get path flag: %w", err)
	}
	outputFormat, outErr := cmd.Flags().GetString("output")
	if outErr != nil {
		return fmt.Errorf("failed to get output flag: %w", outErr)
	}
	minSeverity, msErr := cmd.Flags().GetString("min-severity")
	if msErr != nil {
		return fmt.Errorf("failed to get min-severity flag: %w", msErr)
	}
	minRank, ok := severityRank[minSeverity]
	if !ok {
		return fmt.Errorf("invalid --min-severity %q: must be Low, Medium, High, or Critical", minSeverity)
	}
	logger := logging.GetLogger()

	logger.Info().Str("path", path).Msg("Starting linting scan")

	k8sLinter := linter.NewKubernetesLinter()
	var totalIssues []linter.Issue
	var passedFiles int
	var failedFiles int

	walkErr := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(filePath)
		logger.Debug().Str("file", filePath).Msg("Linting file")
		// Check if it's a Chart metadata file
		if filepath.Base(filePath) == "Chart.yaml" {
			helmLinter := linter.NewHelmLinter()
			result, err := helmLinter.Lint(context.Background(), filePath)
			if err != nil {
				logger.Error().Err(err).Str("file", filePath).Msg("Failed to lint Helm chart")
				return nil
			}
			processResult(result, &passedFiles, &failedFiles, &totalIssues)
			return nil
		}

		// Run Kubernetes Linter
		if ext == ".yaml" || ext == ".yml" {
			result, err := k8sLinter.Lint(context.Background(), filePath)
			if err != nil {
				logger.Error().Err(err).Str("file", filePath).Msg("Failed to lint file")
				return nil
			}
			processResult(result, &passedFiles, &failedFiles, &totalIssues)
			return nil
		}

		// Run Dockerfile Linter
		fName := filepath.Base(filePath)
		if fName == "Dockerfile" || strings.HasSuffix(fName, ".Dockerfile") || strings.HasSuffix(fName, ".dockerfile") {
			dockerLinter := linter.NewDockerfileLinter()
			result, err := dockerLinter.Lint(context.Background(), filePath)
			if err != nil {
				logger.Error().Err(err).Str("file", filePath).Msg("Failed to lint file")
				return nil
			}
			processResult(result, &passedFiles, &failedFiles, &totalIssues)
			return nil
		}

		// Run Terraform Linter
		if ext == ".tf" {
			tfLinter := linter.NewTerraformLinter()
			result, err := tfLinter.Lint(context.Background(), filePath)
			if err != nil {
				logger.Error().Err(err).Str("file", filePath).Msg("Failed to lint Terraform file")
				return nil
			}
			processResult(result, &passedFiles, &failedFiles, &totalIssues)
			return nil
		}

		return nil
	})

	if walkErr != nil {
		return fmt.Errorf("error walking path: %w", walkErr)
	}

	// Filter issues by min severity
	var filteredIssues []linter.Issue
	for _, issue := range totalIssues {
		if severityRank[issue.Severity] >= minRank {
			filteredIssues = append(filteredIssues, issue)
		}
	}

	// Build report
	report := LintReport{
		Path:        path,
		FilesPassed: passedFiles,
		FilesFailed: failedFiles,
		TotalIssues: len(filteredIssues),
		Issues:      filteredIssues,
	}

	// Output based on format
	if outputFormat == "json" {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
	} else {
		fmt.Println("\n--- Lint Report ---")
		fmt.Printf("Scanned Path:  %s\n", path)
		fmt.Printf("Files Passed:  %d\n", passedFiles)
		fmt.Printf("Files Failed:  %d\n", failedFiles)
		fmt.Printf("Total Issues:  %d", len(totalIssues))
		if minSeverity != "Low" {
			fmt.Printf(" (showing %s+ only: %d)", minSeverity, len(filteredIssues))
		}
		fmt.Println()

		if len(filteredIssues) > 0 {
			fmt.Println()
			fmt.Printf("%-10s | %-35s | %s\n", "SEVERITY", "FILE", "MESSAGE")
			fmt.Println(strings.Repeat("-", 90))
			for _, issue := range filteredIssues {
				shortFile := issue.File
				if len(shortFile) > 35 {
					shortFile = "..." + shortFile[len(shortFile)-32:]
				}
				fmt.Printf("%-10s | %-35s | %s\n", issue.Severity, shortFile, issue.Message)
			}
			fmt.Println()
		}
	}

	if len(filteredIssues) > 0 {
		return fmt.Errorf("linting failed with %d issues", len(filteredIssues))
	}

	logger.Info().Msg("Linting completed successfully")
	return nil
}

func processResult(result *linter.Result, passed, failed *int, issues *[]linter.Issue) {
	if result == nil {
		return
	}
	if result.Passed {
		*passed++
	} else {
		*failed++
		*issues = append(*issues, result.Issues...)
	}
}

