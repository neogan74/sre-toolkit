package linter

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// TerraformLinter checks Terraform (.tf) files for security and best-practice issues.
// It uses a line-by-line approach with block-tracking so context-sensitive rules
// work correctly (e.g. "open ingress" requires both port AND cidr in the same block).
type TerraformLinter struct{}

func NewTerraformLinter() *TerraformLinter {
	return &TerraformLinter{}
}

var (
	reHardcodedAWSKey  = regexp.MustCompile(`(?i)(access_key|aws_access_key_id)\s*=\s*"AKIA[A-Z0-9]{16}"`)
	reHardcodedSecret  = regexp.MustCompile(`(?i)(secret_key|aws_secret_access_key|password|private_key)\s*=\s*"[^$][^"]{7,}"`)
	reHardcodedToken   = regexp.MustCompile(`(?i)(token|api_key|auth_token|client_secret)\s*=\s*"[^$][^"]{7,}"`)
	reOpenCIDRv4       = regexp.MustCompile(`"0\.0\.0\.0/0"`)
	reOpenCIDRv6       = regexp.MustCompile(`"::/0"`)
	rePublicACL        = regexp.MustCompile(`(?i)acl\s*=\s*"(public-read|public-read-write|authenticated-read)"`)
	reProviderSource   = regexp.MustCompile(`^\s*source\s*=\s*"`)
	reProviderVersion  = regexp.MustCompile(`^\s*version\s*=`)
	reBackendLocal     = regexp.MustCompile(`^\s*backend\s+"local"`)
	reEncryptedFalse   = regexp.MustCompile(`(?i)\bencrypted\s*=\s*false`)
	reStorageEncFalse  = regexp.MustCompile(`(?i)storage_encrypted\s*=\s*false`)
	reFromPort         = regexp.MustCompile(`from_port\s*=\s*(\d+)`)
	reToPort           = regexp.MustCompile(`to_port\s*=\s*(\d+)`)
	reProtocol         = regexp.MustCompile(`protocol\s*=\s*"([^"]+)"`)
	reSourceRanges     = regexp.MustCompile(`source_ranges\s*=\s*\[`)
	reProviderName     = regexp.MustCompile(`^\s*([a-zA-Z0-9_/-]+)\s*=\s*\{`)

	sensitivePortNums = map[string]string{
		"22":    "SSH",
		"3389":  "RDP",
		"1433":  "MSSQL",
		"3306":  "MySQL",
		"5432":  "PostgreSQL",
		"6379":  "Redis",
		"9200":  "Elasticsearch",
		"27017": "MongoDB",
	}
)

// blockContext is one entry on the block stack.
type blockContext struct {
	kind      string // "resource", "ingress", "required_providers", "backend", …
	label     string // first quoted label, e.g. "aws_security_group"
	startLine int

	// Accumulated state inside this block (used when the block closes).
	fromPort    string
	toPort      string
	protocol    string
	hasOpenCIDR bool
}

func (l *TerraformLinter) Lint(_ context.Context, path string) (*Result, error) {
	result := &Result{Passed: true}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	var stack []*blockContext

	// provider version tracking (populated inside required_providers)
	providerVersions := map[string]bool{}  // name → has version
	inRequiredProviders := false
	curProviderName := ""
	curProviderHasVersion := false

	push := func(b *blockContext) { stack = append(stack, b) }

	pop := func() *blockContext {
		if len(stack) == 0 {
			return nil
		}
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return b
	}

	top := func() *blockContext {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1]
	}

	addIssue := func(severity, msg string, line int) {
		result.Issues = append(result.Issues, Issue{
			Severity: severity,
			Message:  msg,
			File:     path,
			Line:     line,
		})
	}

	ingressOrSG := func() bool {
		for _, b := range stack {
			if b.kind == "ingress" {
				return true
			}
		}
		return false
	}

	resourceType := func() string {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].kind == "resource" {
				return stack[i].label
			}
		}
		return ""
	}

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// ── Track attributes inside current block ────────────────────────────
		if t := top(); t != nil {
			if m := reFromPort.FindStringSubmatch(line); len(m) > 1 {
				t.fromPort = m[1]
			}
			if m := reToPort.FindStringSubmatch(line); len(m) > 1 {
				t.toPort = m[1]
			}
			if m := reProtocol.FindStringSubmatch(line); len(m) > 1 {
				t.protocol = m[1]
			}
			if reOpenCIDRv4.MatchString(line) || reOpenCIDRv6.MatchString(line) {
				t.hasOpenCIDR = true
			}
			// GCP source_ranges line that also contains open CIDR
			if reSourceRanges.MatchString(line) && (reOpenCIDRv4.MatchString(line) || reOpenCIDRv6.MatchString(line)) {
				t.hasOpenCIDR = true
			}
		}

		// ── required_providers: track provider version presence ──────────────
		if inRequiredProviders {
			if reProviderVersion.MatchString(line) {
				curProviderHasVersion = true
			}
		}

		// ── Line-level rules (don't need block context) ──────────────────────

		if reHardcodedAWSKey.MatchString(line) {
			addIssue("Critical", "Hardcoded AWS access key detected. Use environment variables or IAM roles.", lineNum)
		}
		if reHardcodedSecret.MatchString(line) {
			if !strings.Contains(line, "var.") && !strings.Contains(line, "data.") && !strings.Contains(line, "local.") {
				addIssue("Critical", "Possible hardcoded secret or password. Use variables or a secrets manager.", lineNum)
			}
		}
		if reHardcodedToken.MatchString(line) {
			if !strings.Contains(line, "var.") && !strings.Contains(line, "data.") && !strings.Contains(line, "local.") {
				addIssue("Critical", "Possible hardcoded token or API key. Use variables or a secrets manager.", lineNum)
			}
		}
		if rePublicACL.MatchString(line) {
			addIssue("High", "Public ACL on storage bucket. Ensure this is intentional; prefer block_public_acls = true.", lineNum)
		}
		if reEncryptedFalse.MatchString(line) {
			addIssue("High", "Encryption is explicitly disabled. Enable encryption at rest.", lineNum)
		}
		if reStorageEncFalse.MatchString(line) {
			addIssue("High", "RDS storage encryption is disabled. Set storage_encrypted = true.", lineNum)
		}
		if reBackendLocal.MatchString(line) {
			addIssue("Medium", `Terraform state stored locally ("local" backend). Use a remote backend (S3, GCS, Terraform Cloud) for shared environments.`, lineNum)
		}

		// GCP firewall open CIDR (source_ranges level, outside ingress blocks)
		if resourceType() == "google_compute_firewall" && (reOpenCIDRv4.MatchString(line) || reOpenCIDRv6.MatchString(line)) {
			addIssue("High", "GCP firewall rule allows traffic from the internet (0.0.0.0/0). Restrict source_ranges.", lineNum)
		}

		// ── Block open ───────────────────────────────────────────────────────
		opens := strings.Count(raw, "{")
		closes := strings.Count(raw, "}")

		for i := 0; i < opens; i++ {
			b := parseBlockHeader(line, lineNum)
			push(b)

			if b.kind == "required_providers" {
				inRequiredProviders = true
			}
			// New provider sub-block inside required_providers
			if inRequiredProviders && b.kind == "provider_entry" {
				if curProviderName != "" {
					providerVersions[curProviderName] = curProviderHasVersion
				}
				curProviderName = b.label
				curProviderHasVersion = false
			}
		}

		// ── Block close ──────────────────────────────────────────────────────
		for i := 0; i < closes; i++ {
			b := pop()
			if b == nil {
				continue
			}

			switch b.kind {
			case "required_providers":
				inRequiredProviders = false
				if curProviderName != "" {
					providerVersions[curProviderName] = curProviderHasVersion
					curProviderName = ""
					curProviderHasVersion = false
				}

			case "provider_entry":
				if inRequiredProviders {
					providerVersions[b.label] = curProviderHasVersion
					curProviderName = ""
					curProviderHasVersion = false
				}

			case "ingress":
				if b.hasOpenCIDR {
					port := b.fromPort
					proto := b.protocol
					switch {
					case proto == "-1" || port == "0":
						addIssue("Critical", "Security group allows ALL traffic from the internet (0.0.0.0/0).", b.startLine)
					default:
						if svc, dangerous := sensitivePortNums[port]; dangerous {
							addIssue("Critical", fmt.Sprintf(
								"Security group allows %s (port %s) from the internet (0.0.0.0/0). Restrict to known IP ranges.",
								svc, port,
							), b.startLine)
						} else if port != "" {
							addIssue("High", fmt.Sprintf(
								"Security group allows port %s from the internet (0.0.0.0/0). Verify this is intentional.",
								port,
							), b.startLine)
						} else {
							addIssue("High", "Open CIDR (0.0.0.0/0) in security group ingress. Verify this is intentional.", b.startLine)
						}
					}
				}

			case "resource":
				// For aws_security_group_rule resources (inline style, no ingress sub-block)
				label := b.label
				if (label == "aws_security_group_rule") && b.hasOpenCIDR && ingressOrSG() {
					addIssue("High", "Security group rule allows traffic from the internet (0.0.0.0/0).", b.startLine)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// ── Post-scan: provider version constraints ───────────────────────────────
	for provider, hasVersion := range providerVersions {
		if !hasVersion {
			addIssue("Medium", fmt.Sprintf(
				"Provider %q has no version constraint. Pin provider versions to avoid unexpected upgrades.",
				provider,
			), 0)
		}
	}

	if len(result.Issues) > 0 {
		result.Passed = false
	}
	return result, nil
}

// parseBlockHeader infers a blockContext from the opening line of a block.
func parseBlockHeader(line string, lineNum int) *blockContext {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return &blockContext{kind: "unknown", startLine: lineNum}
	}
	keyword := fields[0]
	label := ""
	if len(fields) >= 2 {
		label = strings.Trim(fields[1], `"`)
	}

	switch keyword {
	case "resource", "provider", "module", "data", "terraform", "locals", "backend":
		return &blockContext{kind: keyword, label: label, startLine: lineNum}
	case "required_providers":
		return &blockContext{kind: "required_providers", startLine: lineNum}
	case "ingress":
		return &blockContext{kind: "ingress", startLine: lineNum}
	case "egress":
		return &blockContext{kind: "egress", startLine: lineNum}
	default:
		// Check if it looks like a provider entry inside required_providers: `aws = {`
		if m := reProviderName.FindStringSubmatch(line); len(m) > 1 {
			parts := strings.Split(m[1], "/")
			return &blockContext{kind: "provider_entry", label: parts[len(parts)-1], startLine: lineNum}
		}
		return &blockContext{kind: keyword, label: label, startLine: lineNum}
	}
}
