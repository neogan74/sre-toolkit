package main

import (
	"strings"
	"testing"
)

func TestMockCmdRejectsMissingTimeoutDuration(t *testing.T) {
	cmd := newMockCmd()
	cmd.SetArgs([]string{"--timeout-rate", "10"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected command to reject timeout-rate without timeout-duration")
	}
	if !strings.Contains(err.Error(), "--timeout-duration > 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockCmdRejectsInvalidTimeoutRate(t *testing.T) {
	cmd := newMockCmd()
	cmd.SetArgs([]string{"--timeout-rate", "101", "--timeout-duration", "10s"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected command to reject invalid timeout-rate")
	}
	if !strings.Contains(err.Error(), "timeout-rate must be between 0 and 100") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockCmdRejectsInvalidErrorRate(t *testing.T) {
	cmd := newMockCmd()
	cmd.SetArgs([]string{"--error-rate", "-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected command to reject negative error-rate")
	}
	if !strings.Contains(err.Error(), "error-rate must be between 0 and 100") {
		t.Fatalf("unexpected error: %v", err)
	}
}
