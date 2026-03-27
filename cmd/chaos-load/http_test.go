package main

import (
	"strings"
	"testing"
)

func TestHTTPCmdRejectsConflictingAuthFlags(t *testing.T) {
	cmd := newHTTPCmd()
	cmd.SetArgs([]string{
		"--url", "https://example.com",
		"--bearer-token", "token-123",
		"--basic-username", "demo",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected command to reject conflicting auth flags")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPCmdRejectsBasicPasswordWithoutUsername(t *testing.T) {
	cmd := newHTTPCmd()
	cmd.SetArgs([]string{
		"--url", "https://example.com",
		"--basic-password", "secret",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected command to reject missing basic username")
	}
	if !strings.Contains(err.Error(), "--basic-username") {
		t.Fatalf("unexpected error: %v", err)
	}
}
