package inventory

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cert(host, issuer string, status scanner.Status, daysLeft int, notAfter time.Time) *scanner.CertInfo {
	return &scanner.CertInfo{
		Host:     host,
		Issuer:   issuer,
		Status:   status,
		DaysLeft: daysLeft,
		NotAfter: notAfter,
	}
}

func TestBuild_Empty(t *testing.T) {
	rep := Build(nil)
	assert.Equal(t, 0, rep.Total)
	assert.Empty(t, rep.Groups)
}

func TestBuild_SkipsNil(t *testing.T) {
	now := time.Now()
	rep := Build([]*scanner.CertInfo{
		nil,
		cert("a.example.com", "Le CA", scanner.StatusOK, 90, now.Add(90*24*time.Hour)),
		nil,
	})
	assert.Equal(t, 1, rep.Total)
	require.Len(t, rep.Groups, 1)
	assert.Equal(t, 1, rep.Groups[0].Total)
}

func TestBuild_GroupsByIssuerAndCountsStatuses(t *testing.T) {
	now := time.Now()
	certs := []*scanner.CertInfo{
		cert("a.example.com", "Lets Encrypt", scanner.StatusOK, 80, now.Add(80*24*time.Hour)),
		cert("b.example.com", "Lets Encrypt", scanner.StatusWarning, 20, now.Add(20*24*time.Hour)),
		cert("c.example.com", "Lets Encrypt", scanner.StatusCritical, 3, now.Add(3*24*time.Hour)),
		cert("d.example.com", "DigiCert", scanner.StatusExpired, -5, now.Add(-5*24*time.Hour)),
	}

	rep := Build(certs)

	assert.Equal(t, 4, rep.Total)
	assert.Equal(t, 1, rep.OK)
	assert.Equal(t, 1, rep.Warning)
	assert.Equal(t, 1, rep.Critical)
	assert.Equal(t, 1, rep.Expired)
	require.Len(t, rep.Groups, 2)

	byIssuer := map[string]Group{}
	for _, g := range rep.Groups {
		byIssuer[g.Issuer] = g
	}

	le := byIssuer["Lets Encrypt"]
	assert.Equal(t, 3, le.Total)
	assert.Equal(t, 1, le.OK)
	assert.Equal(t, 1, le.Warning)
	assert.Equal(t, 1, le.Critical)
	// Soonest within Lets Encrypt is c.example.com (3 days).
	assert.Equal(t, "c.example.com", le.SoonestHost)
	assert.Equal(t, 3, le.SoonestDays)

	dc := byIssuer["DigiCert"]
	assert.Equal(t, 1, dc.Total)
	assert.Equal(t, 1, dc.Expired)
	assert.Equal(t, "d.example.com", dc.SoonestHost)
}

func TestBuild_SortsByUrgency(t *testing.T) {
	now := time.Now()
	certs := []*scanner.CertInfo{
		cert("far.example.com", "Far CA", scanner.StatusOK, 200, now.Add(200*24*time.Hour)),
		cert("soon.example.com", "Soon CA", scanner.StatusCritical, 2, now.Add(2*24*time.Hour)),
		cert("mid.example.com", "Mid CA", scanner.StatusWarning, 25, now.Add(25*24*time.Hour)),
	}

	rep := Build(certs)
	require.Len(t, rep.Groups, 3)
	assert.Equal(t, "Soon CA", rep.Groups[0].Issuer)
	assert.Equal(t, "Mid CA", rep.Groups[1].Issuer)
	assert.Equal(t, "Far CA", rep.Groups[2].Issuer)
}

func TestBuild_UnknownIssuerAndErrorsSortLast(t *testing.T) {
	now := time.Now()
	certs := []*scanner.CertInfo{
		// Connection error: no issuer, no expiry.
		cert("broken.example.com", "", scanner.StatusError, 0, time.Time{}),
		cert("good.example.com", "Good CA", scanner.StatusOK, 50, now.Add(50*24*time.Hour)),
	}

	rep := Build(certs)
	require.Len(t, rep.Groups, 2)
	// Group with a real expiry sorts before the error-only group.
	assert.Equal(t, "Good CA", rep.Groups[0].Issuer)
	assert.Equal(t, unknownIssuer, rep.Groups[1].Issuer)
	assert.Equal(t, 1, rep.Errors)
	// Error group seeds SoonestHost but has no expiry timestamp.
	assert.True(t, rep.Groups[1].SoonestAfter.IsZero())
	assert.Equal(t, "broken.example.com", rep.Groups[1].SoonestHost)
}

func TestBuild_DeterministicTieBreak(t *testing.T) {
	now := time.Now()
	exp := now.Add(30 * 24 * time.Hour)
	// Two issuers whose soonest expiry is identical: order must be by issuer name.
	certs := []*scanner.CertInfo{
		cert("z.example.com", "Zeta CA", scanner.StatusWarning, 30, exp),
		cert("a.example.com", "Alpha CA", scanner.StatusWarning, 30, exp),
	}

	rep := Build(certs)
	require.Len(t, rep.Groups, 2)
	assert.Equal(t, "Alpha CA", rep.Groups[0].Issuer)
	assert.Equal(t, "Zeta CA", rep.Groups[1].Issuer)
}
