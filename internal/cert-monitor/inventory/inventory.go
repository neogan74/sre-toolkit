// Package inventory aggregates scanned TLS certificates into a grouped
// summary, making it easy to see exposure by issuer and which certificate
// in each group expires soonest.
package inventory

import (
	"sort"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
)

// Group summarizes all certificates issued by a single issuer.
type Group struct {
	Issuer       string    `json:"issuer"`
	Total        int       `json:"total"`
	OK           int       `json:"ok"`
	Warning      int       `json:"warning"`
	Critical     int       `json:"critical"`
	Expired      int       `json:"expired"`
	Errors       int       `json:"errors"`
	SoonestHost  string    `json:"soonest_host"`  // host whose cert expires first in this group
	SoonestDays  int       `json:"soonest_days"`  // days until that expiry (may be negative)
	SoonestAfter time.Time `json:"soonest_after"` // expiry timestamp of the soonest cert
}

// Report is the full inventory: groups sorted by urgency, plus overall totals.
type Report struct {
	Groups   []Group `json:"groups"`
	Total    int     `json:"total"`
	OK       int     `json:"ok"`
	Warning  int     `json:"warning"`
	Critical int     `json:"critical"`
	Expired  int     `json:"expired"`
	Errors   int     `json:"errors"`
}

const unknownIssuer = "(unknown)"

// Build aggregates scan results into an inventory grouped by issuer.
//
// Groups are sorted by urgency: those containing the soonest-expiring valid
// certificate come first. Certificates that errored (no expiry available) sort
// last within and across groups. The issuer label falls back to "(unknown)"
// when empty.
func Build(certs []*scanner.CertInfo) Report {
	groups := make(map[string]*Group)
	var rep Report

	for _, c := range certs {
		if c == nil {
			continue
		}
		rep.Total++
		countStatus(&rep, c.Status)

		issuer := c.Issuer
		if issuer == "" {
			issuer = unknownIssuer
		}

		g, ok := groups[issuer]
		if !ok {
			g = &Group{Issuer: issuer}
			groups[issuer] = g
		}
		g.Total++
		countGroupStatus(g, c.Status)
		updateSoonest(g, c)
	}

	rep.Groups = make([]Group, 0, len(groups))
	for _, g := range groups {
		rep.Groups = append(rep.Groups, *g)
	}
	sortGroups(rep.Groups)
	return rep
}

// updateSoonest records the certificate in a group with the nearest expiry.
// Certificates with no usable expiry (zero time, typically connection errors)
// never displace a real expiry but seed the group if it has nothing yet.
func updateSoonest(g *Group, c *scanner.CertInfo) {
	if c.NotAfter.IsZero() {
		if g.SoonestHost == "" {
			g.SoonestHost = c.Host
		}
		return
	}
	if g.SoonestAfter.IsZero() || c.NotAfter.Before(g.SoonestAfter) {
		g.SoonestHost = c.Host
		g.SoonestAfter = c.NotAfter
		g.SoonestDays = c.DaysLeft
	}
}

func countStatus(rep *Report, s scanner.Status) {
	switch s {
	case scanner.StatusOK:
		rep.OK++
	case scanner.StatusWarning:
		rep.Warning++
	case scanner.StatusCritical:
		rep.Critical++
	case scanner.StatusExpired:
		rep.Expired++
	case scanner.StatusError:
		rep.Errors++
	}
}

func countGroupStatus(g *Group, s scanner.Status) {
	switch s {
	case scanner.StatusOK:
		g.OK++
	case scanner.StatusWarning:
		g.Warning++
	case scanner.StatusCritical:
		g.Critical++
	case scanner.StatusExpired:
		g.Expired++
	case scanner.StatusError:
		g.Errors++
	}
}

// sortGroups orders groups by ascending soonest expiry (most urgent first).
// Groups with no real expiry (only errors) sort last; ties break by issuer
// name for stable, deterministic output.
func sortGroups(groups []Group) {
	sort.Slice(groups, func(i, j int) bool {
		gi, gj := groups[i], groups[j]
		iZero, jZero := gi.SoonestAfter.IsZero(), gj.SoonestAfter.IsZero()
		if iZero != jZero {
			return !iZero // non-zero (has expiry) sorts first
		}
		if !iZero && !gi.SoonestAfter.Equal(gj.SoonestAfter) {
			return gi.SoonestAfter.Before(gj.SoonestAfter)
		}
		return gi.Issuer < gj.Issuer
	})
}
