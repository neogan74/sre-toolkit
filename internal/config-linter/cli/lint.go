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
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func newLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint configuration files",
		Long:  `Scan directories or files for configuration issues in Kubernetes YAMLs, etc.`,
		RunE:  runLint,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to directory or file to lint")
	cmd.Flags().StringP("output", "o", "table", "Output format (table, json)")
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

func runLint(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("path")
	outputFormat, _ := cmd.Flags().GetString("output")
	logger := logging.GetLogger()

	logger.Info().Str("path", path).Msg("Starting linting scan")

	k8sLinter := linter.NewKubernetesLinter()
	var totalIssues []linter.Issue
	var passedFiles int
	var failedFiles int

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}



		ext := filepath.Ext(filePath)
		logger.Debug().Str("file", filePath).Msg("Linting file")

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

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking path: %w", err)
	}

	// Build report
	report := LintReport{
		Path:        path,
		FilesPassed: passedFiles,
		FilesFailed: failedFiles,
		TotalIssues: len(totalIssues),
		Issues:      totalIssues,
	}

	// Output based on format
	if outputFormat == "json" {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
	} else {
		// Table output
		fmt.Println("\n--- Lint Report ---")
		fmt.Printf("Scanned Path: %s\n", path)
		fmt.Printf("Files Passed: %d\n", passedFiles)
		fmt.Printf("Files Failed: %d\n", failedFiles)
		fmt.Printf("Total Issues: %d\n\n", len(totalIssues))

		if len(totalIssues) > 0 {
			fmt.Printf("%-10s | %-30s | %s\n", "SEVERITY", "FILE", "MESSAGE")
			fmt.Println("--------------------------------------------------------------------------------")
			for _, issue := range totalIssues {
				// Truncate file path if too long
				shortFile := issue.File
				if len(shortFile) > 30 {
					shortFile = "..." + shortFile[len(shortFile)-27:]
				}
				fmt.Printf("%-10s | %-30s | %s\n", issue.Severity, shortFile, issue.Message)
			}
			fmt.Println("")
		}
	}

	if len(totalIssues) > 0 {
		return fmt.Errorf("linting failed with %d issues", len(totalIssues))
	}

	logger.Info().Msg("Linting completed successfully")
	return nil
}

func processResult(result *linter.Result, passed *int, failed *int, issues *[]linter.Issue) {
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

// Map zerolog level to string if needed, but we use string in Issue struct.
func severityToLevel(severity string) zerolog.Level {
	switch severity {
	case "High", "Critical":
		return zerolog.ErrorLevel
	case "Medium", "Warning":
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}
