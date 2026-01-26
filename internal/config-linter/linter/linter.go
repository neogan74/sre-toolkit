package linter

import (
	"context"
)

// Linter defines the interface for configuration linters
type Linter interface {
	Lint(ctx context.Context, path string) (*Result, error)
}

// Result holds the result of a linting operation
type Result struct {
	Passed bool
	Issues []Issue
}

// Issue represents a single linting issue
type Issue struct {
	Severity string
	Message  string
	File     string
	Line     int
}
