// Package healthcheck provides functionality for checking the health of Kubernetes components.
package healthcheck

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ComponentStatus represents the health status of a cluster component
type ComponentStatus struct {
	Name    string
	Status  string // Healthy, Unhealthy, Unknown
	Message string
}

// CheckComponents checks the health status of cluster components
func CheckComponents(ctx context.Context, clientset kubernetes.Interface) ([]ComponentStatus, error) {
	// Get API server version for compatibility check
	serverVersion := "unknown"
	if versionInfo, err := clientset.Discovery().ServerVersion(); err == nil {
		serverVersion = versionInfo.GitVersion
	}

	// Try to get component statuses (deprecated in newer k8s versions)
	components, err := clientset.CoreV1().ComponentStatuses().List(ctx, metav1.ListOptions{})
	if err != nil || len(components.Items) == 0 {
		// If ComponentStatus API is not available or returns no results, check via pods
		return checkComponentPods(ctx, clientset, serverVersion)
	}

	statuses := []ComponentStatus{} // Initialize as empty slice, not nil
	for _, comp := range components.Items {
		status := ComponentStatus{
			Name:   comp.Name,
			Status: "Unknown",
		}

		for _, condition := range comp.Conditions {
			if condition.Type == "Healthy" {
				if condition.Status == "True" {
					status.Status = "Healthy"
					status.Message = condition.Message
				} else {
					status.Status = "Unhealthy"
					status.Message = condition.Message
					if condition.Error != "" {
						status.Message = condition.Error
					}
				}
				break
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// checkComponentPods checks component health via system pods
func checkComponentPods(ctx context.Context, clientset kubernetes.Interface, apiServerVersion string) ([]ComponentStatus, error) {
	// Check kube-system namespace for control plane components
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list kube-system pods: %w", err)
	}

	componentMap := make(map[string]*ComponentStatus)
	componentNames := []string{
		"kube-apiserver",
		"kube-controller-manager",
		"kube-scheduler",
		"etcd",
		"coredns",
		"kube-proxy",
	}

	// Initialize component statuses
	for _, name := range componentNames {
		componentMap[name] = &ComponentStatus{
			Name:   name,
			Status: "NotFound",
		}
	}

	// Check pod status for each component
	for _, pod := range pods.Items {
		for _, name := range componentNames {
			if matchesComponent(pod.Name, name) {
				if pod.Status.Phase == "Running" {
					// Check if all containers are ready
					allReady := true
					for _, cs := range pod.Status.ContainerStatuses {
						if !cs.Ready {
							allReady = false
							break
						}
					}
					if allReady {
						componentMap[name].Status = "Healthy"
						componentMap[name].Message = "Pod running and ready"
					} else {
						componentMap[name].Status = "Unhealthy"
						componentMap[name].Message = "Pod running but not ready"
					}
				} else {
					componentMap[name].Status = "Unhealthy"
					componentMap[name].Message = fmt.Sprintf("Pod in %s phase", pod.Status.Phase)
				}

				// Check version compatibility if possible
				if apiServerVersion != "unknown" && len(pod.Spec.Containers) > 0 {
					image := pod.Spec.Containers[0].Image
					if version := extractVersionFromImage(image); version != "" {
						apiVersion, err1 := ParseVersion(apiServerVersion)
						compVersion, err2 := ParseVersion(version)
						if err1 == nil && err2 == nil {
							if skewIssue := GetVersionSkewDescription(compVersion, apiVersion, name); skewIssue != "" {
								if componentMap[name].Status == "Healthy" {
									componentMap[name].Status = "Warning"
								}
								componentMap[name].Message = fmt.Sprintf("%s. %s", componentMap[name].Message, skewIssue)
							}
						}
					}
				}
			}
		}
	}

	// Convert map to slice
	statuses := []ComponentStatus{} // Initialize as empty slice, not nil
	for _, name := range componentNames {
		if componentMap[name].Status != "NotFound" {
			statuses = append(statuses, *componentMap[name])
		}
	}

	return statuses, nil
}

// matchesComponent checks if a pod name matches a component name
func matchesComponent(podName, componentName string) bool {
	// Simple prefix matching (could be improved)
	return len(podName) >= len(componentName) && podName[:len(componentName)] == componentName
}

// extractVersionFromImage attempts to extract a version tag from an image string
func extractVersionFromImage(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return ""
	}
	tag := parts[len(parts)-1]
	// Basic validation that tag looks like a version
	if versionRegex.MatchString(tag) {
		return tag
	}
	return ""
}
