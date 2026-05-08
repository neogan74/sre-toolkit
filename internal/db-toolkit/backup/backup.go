// Package backup provides database backup automation.
package backup

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/neogan/sre-toolkit/internal/db-toolkit/connector"
)

// Config holds backup parameters.
type Config struct {
	OutputDir  string
	Compress   bool
	FilePrefix string
}

// Result describes the outcome of a backup operation.
type Result struct {
	FilePath   string
	Size       int64
	Duration   time.Duration
	Compressed bool
}

// Run creates a database backup using the appropriate tool (pg_dump / mysqldump).
func Run(ctx context.Context, dbCfg *connector.Config, bkpCfg *Config) (*Result, error) {
	if err := os.MkdirAll(bkpCfg.OutputDir, 0750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	ts := time.Now().Format("20060102_150405")
	prefix := bkpCfg.FilePrefix
	if prefix == "" {
		prefix = string(dbCfg.Type)
	}
	fileName := fmt.Sprintf("%s_%s_%s.sql", prefix, dbCfg.Database, ts)
	if bkpCfg.Compress {
		fileName += ".gz"
	}
	filePath := filepath.Join(bkpCfg.OutputDir, fileName)

	start := time.Now()

	var err error
	switch dbCfg.Type {
	case connector.PostgreSQL:
		err = pgDump(ctx, dbCfg, filePath, bkpCfg.Compress)
	case connector.MySQL:
		err = mysqlDump(ctx, dbCfg, filePath, bkpCfg.Compress)
	default:
		return nil, fmt.Errorf("unsupported db type: %s", dbCfg.Type)
	}
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat output file: %w", err)
	}

	return &Result{
		FilePath:   filePath,
		Size:       info.Size(),
		Duration:   time.Since(start),
		Compressed: bkpCfg.Compress,
	}, nil
}

func pgDump(ctx context.Context, cfg *connector.Config, outPath string, compress bool) error {
	tool, err := exec.LookPath("pg_dump")
	if err != nil {
		return fmt.Errorf("pg_dump not found in PATH: %w", err)
	}

	args := []string{
		"-h", cfg.Host,
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-U", cfg.User,
		"-d", cfg.Database,
		"--no-password",
	}

	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.Password))

	return runDump(cmd, outPath, compress)
}

func mysqlDump(ctx context.Context, cfg *connector.Config, outPath string, compress bool) error {
	tool, err := exec.LookPath("mysqldump")
	if err != nil {
		return fmt.Errorf("mysqldump not found in PATH: %w", err)
	}

	args := []string{
		fmt.Sprintf("--host=%s", cfg.Host),
		fmt.Sprintf("--port=%d", cfg.Port),
		fmt.Sprintf("--user=%s", cfg.User),
		fmt.Sprintf("--password=%s", cfg.Password),
		"--single-transaction",
		"--routines",
		"--triggers",
		cfg.Database,
	}

	cmd := exec.CommandContext(ctx, tool, args...)
	return runDump(cmd, outPath, compress)
}

func runDump(cmd *exec.Cmd, outPath string, compress bool) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	var w io.Writer = f
	if compress {
		gz := gzip.NewWriter(f)
		defer gz.Close()
		w = gz
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start dump: %w", err)
	}

	if _, err := io.Copy(w, stdout); err != nil {
		return fmt.Errorf("copy dump output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("dump failed: %w\nstderr: %s", err, stderrBuf.String())
	}

	return nil
}
