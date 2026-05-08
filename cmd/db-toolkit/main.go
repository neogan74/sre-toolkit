// Package main provides the entry point for the db-toolkit tool.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/neogan/sre-toolkit/internal/db-toolkit/analyzer"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/backup"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/connector"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/health"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/reporter"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	logging.Init(nil)

	rootCmd := &cobra.Command{
		Use:   "db-toolkit",
		Short: "Database operations helper for SRE teams",
		Long: `db-toolkit automates routine database tasks and provides health monitoring,
performance analysis, and backup automation for PostgreSQL and MySQL.`,
	}

	// Global flags
	rootCmd.PersistentFlags().String("type", "postgres", "Database type (postgres, mysql)")
	rootCmd.PersistentFlags().String("host", "localhost", "Database host")
	rootCmd.PersistentFlags().Int("port", 0, "Database port (default: 5432 for postgres, 3306 for mysql)")
	rootCmd.PersistentFlags().String("user", "", "Database user")
	rootCmd.PersistentFlags().String("password", "", "Database password (or use env DB_PASSWORD)")
	rootCmd.PersistentFlags().String("database", "", "Database name")
	rootCmd.PersistentFlags().String("sslmode", "disable", "SSL mode for PostgreSQL (disable, require, verify-full)")
	rootCmd.PersistentFlags().String("output", "table", "Output format (table, json)")

	_ = viper.BindPFlag("type", rootCmd.PersistentFlags().Lookup("type"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	_ = viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	_ = viper.BindPFlag("user", rootCmd.PersistentFlags().Lookup("user"))
	_ = viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	_ = viper.BindPFlag("database", rootCmd.PersistentFlags().Lookup("database"))
	_ = viper.BindPFlag("sslmode", rootCmd.PersistentFlags().Lookup("sslmode"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	viper.SetEnvPrefix("DB")
	viper.AutomaticEnv()

	rootCmd.AddCommand(
		newHealthCmd(),
		newAnalyzeCmd(),
		newBackupCmd(),
		newQueryCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildConnectorConfig() *connector.Config {
	dbType := connector.DBType(viper.GetString("type"))
	port := viper.GetInt("port")
	if port == 0 {
		if dbType == connector.MySQL {
			port = 3306
		} else {
			port = 5432
		}
	}

	password := viper.GetString("password")
	if password == "" {
		password = os.Getenv("DB_PASSWORD")
	}

	return &connector.Config{
		Type:            dbType,
		Host:            viper.GetString("host"),
		Port:            port,
		User:            viper.GetString("user"),
		Password:        password,
		Database:        viper.GetString("database"),
		SSLMode:         viper.GetString("sslmode"),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

func outputFormat() reporter.Format {
	if viper.GetString("output") == "json" {
		return reporter.FormatJSON
	}
	return reporter.FormatTable
}

// newHealthCmd creates the `health` subcommand.
func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check database health",
		Long:  `Connects to the database and runs health checks: connectivity, connections, size, replication lag, slow queries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := buildConnectorConfig()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			report, err := health.Run(ctx, cfg)
			if err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}

			reporter.PrintHealthReport(os.Stdout, report, outputFormat())

			if report.Overall == health.StatusCritical {
				os.Exit(2)
			}
			return nil
		},
	}
}

// newAnalyzeCmd creates the `analyze` subcommand.
func newAnalyzeCmd() *cobra.Command {
	var topN int
	var minQueryMs int
	var noIndexes bool

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze database performance",
		Long:  `Analyzes table sizes, slow queries (requires pg_stat_statements for PostgreSQL), and index usage.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := buildConnectorConfig()
			opts := &analyzer.Config{
				TopN:           topN,
				MinQueryMs:     minQueryMs,
				IncludeIndexes: !noIndexes,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			report, err := analyzer.Analyze(ctx, cfg, opts)
			if err != nil {
				return fmt.Errorf("analysis failed: %w", err)
			}

			reporter.PrintAnalysisReport(os.Stdout, report, outputFormat())
			return nil
		},
	}

	cmd.Flags().IntVar(&topN, "top", 10, "Number of top items to show")
	cmd.Flags().IntVar(&minQueryMs, "min-query-ms", 100, "Minimum mean query time (ms) for slow query list")
	cmd.Flags().BoolVar(&noIndexes, "no-indexes", false, "Skip index analysis")
	return cmd
}

// newBackupCmd creates the `backup` subcommand.
func newBackupCmd() *cobra.Command {
	var outputDir string
	var compress bool
	var prefix string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a database backup",
		Long:  `Creates a backup using pg_dump (PostgreSQL) or mysqldump (MySQL). Supports optional gzip compression.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := buildConnectorConfig()
			bkpCfg := &backup.Config{
				OutputDir:  outputDir,
				Compress:   compress,
				FilePrefix: prefix,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			result, err := backup.Run(ctx, cfg, bkpCfg)
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			reporter.PrintBackupResult(os.Stdout, result, outputFormat())
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Directory to store backup files")
	cmd.Flags().BoolVar(&compress, "compress", true, "Compress backup with gzip")
	cmd.Flags().StringVar(&prefix, "prefix", "", "File prefix (default: db type)")
	return cmd
}

// newQueryCmd creates the `query` subcommand.
func newQueryCmd() *cobra.Command {
	var queryStr string
	var maxRows int

	cmd := &cobra.Command{
		Use:   "query [SQL]",
		Short: "Execute a SQL query and display results",
		Long:  `Connects to the database, executes the given SQL query, and prints results in table or JSON format.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if queryStr == "" && len(args) > 0 {
				queryStr = args[0]
			}
			if queryStr == "" {
				return fmt.Errorf("query is required (use --query or pass as argument)")
			}

			cfg := buildConnectorConfig()
			db, err := connector.Connect(cfg)
			if err != nil {
				return fmt.Errorf("connection failed: %w", err)
			}
			defer db.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			rows, err := db.QueryContext(ctx, queryStr)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			if outputFormat() == reporter.FormatJSON {
				return printQueryJSON(os.Stdout, rows, cols, maxRows)
			}
			return printQueryTable(os.Stdout, rows, cols, maxRows)
		},
	}

	cmd.Flags().StringVarP(&queryStr, "query", "q", "", "SQL query to execute")
	cmd.Flags().IntVar(&maxRows, "max-rows", 100, "Maximum rows to display (0 = unlimited)")
	return cmd
}

func printQueryTable(w *os.File, rows *sql.Rows, cols []string, maxRows int) error {
	fmt.Fprintln(w)
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-20s", c)
	}
	fmt.Fprintln(w)
	for range cols {
		fmt.Fprintf(w, "%-20s  ", "--------------------")
	}
	fmt.Fprintln(w)

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	count := 0
	for rows.Next() {
		if maxRows > 0 && count >= maxRows {
			fmt.Fprintf(w, "... (limited to %d rows)\n", maxRows)
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		for i, v := range vals {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			fmt.Fprintf(w, "%-20v", v)
		}
		fmt.Fprintln(w)
		count++
	}
	fmt.Fprintf(w, "\n(%d rows)\n", count)
	return rows.Err()
}

func printQueryJSON(w *os.File, rows *sql.Rows, cols []string, maxRows int) error {
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	var result []map[string]any
	count := 0
	for rows.Next() {
		if maxRows > 0 && count >= maxRows {
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = vals[i]
		}
		result = append(result, row)
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("db-toolkit v0.7.0")
		},
	}
}
