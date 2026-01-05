package collector

import "time"

// Alert represents a single alert instance
type Alert struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"` // firing, pending, inactive
	Value       float64           `json:"value"`
	ActiveAt    time.Time         `json:"active_at"`
	FiredAt     time.Time         `json:"fired_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
}

// AlertHistory represents a collection of alerts over a time period
type AlertHistory struct {
	Alerts    []Alert   `json:"alerts"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Source    string    `json:"source"` // prometheus, alertmanager
}

// GetAlertName returns the alert name
func (a *Alert) GetAlertName() string {
	return a.Name
}

// GetSeverity returns the severity label value, or "unknown" if not present
func (a *Alert) GetSeverity() string {
	if severity, ok := a.Labels["severity"]; ok {
		return severity
	}
	return "unknown"
}

// GetNamespace returns the namespace label value, or empty string if not present
func (a *Alert) GetNamespace() string {
	if ns, ok := a.Labels["namespace"]; ok {
		return ns
	}
	return ""
}

// GetService returns the service label value, or empty string if not present
func (a *Alert) GetService() string {
	if svc, ok := a.Labels["service"]; ok {
		return svc
	}
	return ""
}

// Duration returns the duration the alert was in firing state
func (a *Alert) Duration() time.Duration {
	if a.ResolvedAt != nil {
		return a.ResolvedAt.Sub(a.FiredAt)
	}
	// If not resolved yet, calculate duration from firing time to now
	return time.Since(a.FiredAt)
}

// IsResolved returns true if the alert has been resolved
func (a *Alert) IsResolved() bool {
	return a.ResolvedAt != nil
}

// IsFiring returns true if the alert is currently firing
func (a *Alert) IsFiring() bool {
	return a.State == "firing"
}

// AlertGroup groups alerts by name for analysis
type AlertGroup struct {
	Name   string
	Alerts []Alert
}

// GroupAlertsByName groups alerts by their alert name
func GroupAlertsByName(alerts []Alert) map[string][]Alert {
	groups := make(map[string][]Alert)
	for _, alert := range alerts {
		name := alert.GetAlertName()
		groups[name] = append(groups[name], alert)
	}
	return groups
}

// CountAlerts returns the total number of alerts in the history
func (h *AlertHistory) CountAlerts() int {
	return len(h.Alerts)
}

// CountUniqueAlerts returns the number of unique alert names
func (h *AlertHistory) CountUniqueAlerts() int {
	unique := make(map[string]bool)
	for _, alert := range h.Alerts {
		unique[alert.GetAlertName()] = true
	}
	return len(unique)
}

// GetAlertNames returns a sorted list of unique alert names
func (h *AlertHistory) GetAlertNames() []string {
	unique := make(map[string]bool)
	for _, alert := range h.Alerts {
		unique[alert.GetAlertName()] = true
	}

	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	return names
}
