package reporter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/inventory"
	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
)

func TestReportInventoryTable(t *testing.T) {
	certs := []*scanner.CertInfo{
		{Host: "a.example.com", Issuer: "Let's Encrypt", NotAfter: time.Now().Add(48 * time.Hour), DaysLeft: 2, Status: scanner.StatusCritical},
		{Host: "b.example.com", Issuer: "Let's Encrypt", NotAfter: time.Now().Add(90 * 24 * time.Hour), DaysLeft: 90, Status: scanner.StatusOK},
	}
	rep := inventory.Build(certs)

	var buf bytes.Buffer
	r := New(FormatTable, &buf)
	if err := r.ReportInventory(rep); err != nil {
		t.Fatalf("ReportInventory() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Let's Encrypt") {
		t.Errorf("expected output to contain issuer name, got:\n%s", out)
	}
	if !strings.Contains(out, "a.example.com") {
		t.Errorf("expected output to contain the soonest-expiring host, got:\n%s", out)
	}
}

func TestReportInventoryJSON(t *testing.T) {
	certs := []*scanner.CertInfo{
		{Host: "a.example.com", Issuer: "Let's Encrypt", NotAfter: time.Now().Add(48 * time.Hour), DaysLeft: 2, Status: scanner.StatusCritical},
	}
	rep := inventory.Build(certs)

	var buf bytes.Buffer
	r := New(FormatJSON, &buf)
	if err := r.ReportInventory(rep); err != nil {
		t.Fatalf("ReportInventory() error = %v", err)
	}

	if !strings.Contains(buf.String(), `"issuer": "Let's Encrypt"`) {
		t.Errorf("expected JSON output to contain issuer field, got:\n%s", buf.String())
	}
}
