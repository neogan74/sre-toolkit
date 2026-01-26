package cli

import (
	"github.com/neogan/sre-toolkit/pkg/cli"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command for config-linter
func NewRootCmd() *cobra.Command {
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "config-linter"
	rootCmd.Short = "Configuration Linter and Validator"
	rootCmd.Long = `config-linter validates various configuration files including
Kubernetes YAML, Helm charts, Terraform, Dockerfiles, and CI/CD configs.
It checks for syntax errors, best practices, and security issues.`

	// Add subcommands here
	// rootCmd.AddCommand(newLintCmd())

	return rootCmd
}

// Execute runs the root command
func Execute() error {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		logger := logging.GetLogger()
		logger.Error().Err(err).Msg("Command execution failed")
		return err
	}
	return nil
}
