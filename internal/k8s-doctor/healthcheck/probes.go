package healthcheck

import (
	corev1 "k8s.io/api/core/v1"
)

// ProbeIssue represents a pod with probe configuration issues
type ProbeIssue struct {
	Pod        string
	Namespace  string
	Containers []ContainerProbeIssue
}

// ContainerProbeIssue represents probe issues for a specific container
type ContainerProbeIssue struct {
	Name   string
	Issues []string
}

// AuditPodProbes checks if pod containers have liveness/readiness probes configured
// Returns nil if all containers have both probes configured
func AuditPodProbes(pod *corev1.Pod) *ProbeIssue {
	issue := &ProbeIssue{
		Pod:        pod.Name,
		Namespace:  pod.Namespace,
		Containers: []ContainerProbeIssue{},
	}

	for _, container := range pod.Spec.Containers {
		containerIssue := ContainerProbeIssue{
			Name:   container.Name,
			Issues: []string{},
		}

		if container.LivenessProbe == nil {
			containerIssue.Issues = append(containerIssue.Issues, "Liveness probe not configured")
		}

		if container.ReadinessProbe == nil {
			containerIssue.Issues = append(containerIssue.Issues, "Readiness probe not configured")
		}

		// Only add container to issues if it has problems
		if len(containerIssue.Issues) > 0 {
			issue.Containers = append(issue.Containers, containerIssue)
		}
	}

	// Return nil if no issues found
	if len(issue.Containers) > 0 {
		return issue
	}
	return nil
}
