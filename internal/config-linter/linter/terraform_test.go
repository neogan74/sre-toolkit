package linter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTF writes content to a temporary .tf file and returns its path.
func writeTF(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.tf")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestTerraformLinter_Clean(t *testing.T) {
	tf := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  backend "s3" {
    bucket = "my-terraform-state"
    key    = "prod/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_security_group" "web" {
  name = "web-sg"

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "expected clean config to pass, got issues: %v", result.Issues)
}

func TestTerraformLinter_OpenSSHIngress(t *testing.T) {
	tf := `
resource "aws_security_group" "bad" {
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "SSH")
}

func TestTerraformLinter_OpenRDPIngress(t *testing.T) {
	tf := `
resource "aws_security_group" "bad" {
  ingress {
    from_port   = 3389
    to_port     = 3389
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "RDP")
}

func TestTerraformLinter_AllTrafficFromInternet(t *testing.T) {
	tf := `
resource "aws_security_group" "bad" {
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "0.0.0.0/0")
}

func TestTerraformLinter_HardcodedAWSKey(t *testing.T) {
	tf := `
provider "aws" {
  access_key = "AKIAIOSFODNN7EXAMPLE"
  secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "access key")
}

func TestTerraformLinter_VariableCredentialsOK(t *testing.T) {
	// Using var.* references should NOT trigger hardcoded credential warnings.
	tf := `
provider "aws" {
  access_key = var.aws_access_key
  secret_key = var.aws_secret_key
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.True(t, result.Passed, "variables should not trigger hardcoded credential warnings")
}

func TestTerraformLinter_PublicS3Bucket(t *testing.T) {
	tf := `
resource "aws_s3_bucket_acl" "bad" {
  bucket = aws_s3_bucket.example.id
  acl    = "public-read"
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "Public ACL")
}

func TestTerraformLinter_EncryptionDisabled(t *testing.T) {
	tf := `
resource "aws_ebs_volume" "bad" {
  availability_zone = "us-east-1a"
  size              = 40
  encrypted         = false
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "ncrypt")
}

func TestTerraformLinter_RDSEncryptionDisabled(t *testing.T) {
	tf := `
resource "aws_db_instance" "bad" {
  engine             = "postgres"
  storage_encrypted  = false
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "ncrypt")
}

func TestTerraformLinter_LocalBackend(t *testing.T) {
	tf := `
terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "local")
}

func TestTerraformLinter_MissingProviderVersion(t *testing.T) {
	tf := `
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "version constraint")
}

func TestTerraformLinter_GCPFirewallOpenCIDR(t *testing.T) {
	tf := `
resource "google_compute_firewall" "allow_ssh" {
  name    = "allow-ssh"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["0.0.0.0/0"]
}
`
	l := NewTerraformLinter()
	result, err := l.Lint(context.Background(), writeTF(t, tf))
	require.NoError(t, err)
	assert.False(t, result.Passed)
	requireIssueContains(t, result, "0.0.0.0/0")
}

// requireIssueContains asserts that at least one issue message contains substr.
func requireIssueContains(t *testing.T, result *Result, substr string) {
	t.Helper()
	for _, issue := range result.Issues {
		if strings.Contains(strings.ToLower(issue.Message), strings.ToLower(substr)) {
			return
		}
	}
	t.Errorf("expected an issue containing %q but got: %v", substr, result.Issues)
}