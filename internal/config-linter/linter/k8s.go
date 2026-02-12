package linter

import (
	"context"
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

// KubernetesLinter implements Linter for K8s YAMLs
type KubernetesLinter struct{}

func NewKubernetesLinter() *KubernetesLinter {
	return &KubernetesLinter{}
}

func (l *KubernetesLinter) Lint(ctx context.Context, path string) (*Result, error) {
	result := &Result{Passed: true}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Simple check if it looks like YAML
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return result, nil // Skip non-YAML files
	}

	// Decode
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(content, nil, nil)
	if err != nil {
		// If we can't decode it as a K8s object, it fails "Schema Validation"
		// But maybe it's just a generic YAML. We'll add a warning.
		// For now, let's treat it as a "Schema Error" if it looks like a K8s file but fails.
		// A simple heuristic: check if "apiVersion" and "kind" exist in text.
		if strings.Contains(string(content), "apiVersion:") && strings.Contains(string(content), "kind:") {
			result.Passed = false
			result.Issues = append(result.Issues, Issue{
				Severity: "Error",
				Message:  fmt.Sprintf("Failed to decode Kubernetes object: %v", err),
				File:     path,
				Line:     1,
			})
		}
		return result, nil
	}

	// Common checks
	switch obj := obj.(type) {
	case *corev1.Pod:
		l.checkPodSpec(result, &obj.Spec, path, "Pod", obj.Name)
	case *appsv1.Deployment:
		l.checkPodSpec(result, &obj.Spec.Template.Spec, path, "Deployment", obj.Name)
	case *appsv1.StatefulSet:
		l.checkPodSpec(result, &obj.Spec.Template.Spec, path, "StatefulSet", obj.Name)
	case *appsv1.DaemonSet:
		l.checkPodSpec(result, &obj.Spec.Template.Spec, path, "DaemonSet", obj.Name)
	}

	if len(result.Issues) > 0 {
		result.Passed = false
	}

	return result, nil
}

func (l *KubernetesLinter) checkPodSpec(result *Result, spec *corev1.PodSpec, path, kind, name string) {
	// Security Checks
	if spec.HostNetwork {
		result.Issues = append(result.Issues, Issue{
			Severity: "High",
			Message:  fmt.Sprintf("%s '%s' uses hostNetwork: true", kind, name),
			File:     path,
		})
	}
	if spec.HostPID {
		result.Issues = append(result.Issues, Issue{
			Severity: "High",
			Message:  fmt.Sprintf("%s '%s' uses hostPID: true", kind, name),
			File:     path,
		})
	}
	if spec.HostIPC {
		result.Issues = append(result.Issues, Issue{
			Severity: "High",
			Message:  fmt.Sprintf("%s '%s' uses hostIPC: true", kind, name),
			File:     path,
		})
	}

	for _, container := range spec.Containers {
		// Privileged Check
		if container.SecurityContext != nil && container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			result.Issues = append(result.Issues, Issue{
				Severity: "High",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' is privileged", container.Name, kind, name),
				File:     path,
			})
		}

		// allowPrivilegeEscalation Check
		if container.SecurityContext == nil || container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation {
			result.Issues = append(result.Issues, Issue{
				Severity: "Medium",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' allows privilege escalation (set allowPrivilegeEscalation: false)", container.Name, kind, name),
				File:     path,
			})
		}

		// runAsNonRoot Check
		if container.SecurityContext == nil || container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' does not enforce runAsNonRoot", container.Name, kind, name),
				File:     path,
			})
		}

		// readOnlyRootFilesystem Check
		if container.SecurityContext == nil || container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' does not use readOnlyRootFilesystem", container.Name, kind, name),
				File:     path,
			})
		}

		// Capabilities Check - dangerous capabilities
		if container.SecurityContext != nil && container.SecurityContext.Capabilities != nil {
			dangerousCaps := []string{"SYS_ADMIN", "NET_ADMIN", "ALL", "SYS_PTRACE", "SYS_MODULE"}
			for _, cap := range container.SecurityContext.Capabilities.Add {
				for _, dangerous := range dangerousCaps {
					if string(cap) == dangerous {
						result.Issues = append(result.Issues, Issue{
							Severity: "High",
							Message:  fmt.Sprintf("Container '%s' in %s '%s' adds dangerous capability: %s", container.Name, kind, name, cap),
							File:     path,
						})
					}
				}
			}
		}

		// Resource Limits Check
		if container.Resources.Limits == nil {
			result.Issues = append(result.Issues, Issue{
				Severity: "Medium",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' has no resource limits", container.Name, kind, name),
				File:     path,
			})
		} else {
			if container.Resources.Limits.Cpu().IsZero() {
				result.Issues = append(result.Issues, Issue{
					Severity: "Medium",
					Message:  fmt.Sprintf("Container '%s' in %s '%s' has no CPU limit", container.Name, kind, name),
					File:     path,
				})
			}
			if container.Resources.Limits.Memory().IsZero() {
				result.Issues = append(result.Issues, Issue{
					Severity: "Medium",
					Message:  fmt.Sprintf("Container '%s' in %s '%s' has no Memory limit", container.Name, kind, name),
					File:     path,
				})
			}
		}

		// Image Tag Check (Best Practice)
		if strings.Contains(container.Image, ":latest") || !strings.Contains(container.Image, ":") {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' uses 'latest' tag or no tag", container.Name, kind, name),
				File:     path,
			})
		}

		// Liveness Probe Check (Best Practice)
		if container.LivenessProbe == nil {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' has no livenessProbe", container.Name, kind, name),
				File:     path,
			})
		}

		// Readiness Probe Check (Best Practice)
		if container.ReadinessProbe == nil {
			result.Issues = append(result.Issues, Issue{
				Severity: "Low",
				Message:  fmt.Sprintf("Container '%s' in %s '%s' has no readinessProbe", container.Name, kind, name),
				File:     path,
			})
		}
	}
}
