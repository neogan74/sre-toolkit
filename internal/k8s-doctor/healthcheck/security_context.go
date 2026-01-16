package healthcheck

import (
	corev1 "k8s.io/api/core/v1"
)

// SecurityContextIssue represents a pod with security context issues
type SecurityContextIssue struct {
	Pod        string
	Namespace  string
	Containers []ContainerSecurityIssue
}

// ContainerSecurityIssue represents security issues for a specific container
type ContainerSecurityIssue struct {
	Name   string
	Issues []string
}

// AuditPodSecurityContext checks pod security context configuration
func AuditPodSecurityContext(pod *corev1.Pod) *SecurityContextIssue {
	issue := &SecurityContextIssue{
		Pod:        pod.Name,
		Namespace:  pod.Namespace,
		Containers: []ContainerSecurityIssue{},
	}

	podRunAsNonRoot := false
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsNonRoot != nil {
		podRunAsNonRoot = *pod.Spec.SecurityContext.RunAsNonRoot
	}

	for _, container := range pod.Spec.Containers {
		containerIssue := ContainerSecurityIssue{
			Name:   container.Name,
			Issues: []string{},
		}

		// Check RunAsNonRoot
		containerRunAsNonRoot := podRunAsNonRoot
		if container.SecurityContext != nil && container.SecurityContext.RunAsNonRoot != nil {
			containerRunAsNonRoot = *container.SecurityContext.RunAsNonRoot
		}

		if !containerRunAsNonRoot {
			containerIssue.Issues = append(containerIssue.Issues, "RunAsNonRoot is not set to true")
		}

		// Check ReadOnlyRootFilesystem
		readOnlyRootFilesystem := false
		if container.SecurityContext != nil && container.SecurityContext.ReadOnlyRootFilesystem != nil {
			readOnlyRootFilesystem = *container.SecurityContext.ReadOnlyRootFilesystem
		}

		if !readOnlyRootFilesystem {
			containerIssue.Issues = append(containerIssue.Issues, "ReadOnlyRootFilesystem is not set to true")
		}

		if len(containerIssue.Issues) > 0 {
			issue.Containers = append(issue.Containers, containerIssue)
		}
	}

	if len(issue.Containers) > 0 {
		return issue
	}
	return nil
}
