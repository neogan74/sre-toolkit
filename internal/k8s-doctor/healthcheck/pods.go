package healthcheck

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodStatus represents the health status of pods
type PodStatus struct {
	Total       int
	Running     int
	Pending     int
	Failed      int
	Succeeded   int
	Unknown     int
	Namespaces  map[string]int
	ProblemPods []ProblemPod
}

// ProblemPod represents a pod with issues
type ProblemPod struct {
	Name      string
	Namespace string
	Status    string
	Reason    string
	Message   string
	Restarts  int32
}

// CheckPods checks the health status of all pods in the cluster
func CheckPods(ctx context.Context, clientset kubernetes.Interface, namespace string) (*PodStatus, error) {
	opts := metav1.ListOptions{}
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	status := &PodStatus{
		Namespaces:  make(map[string]int),
		ProblemPods: []ProblemPod{},
	}

	for _, pod := range pods.Items {
		status.Total++
		status.Namespaces[pod.Namespace]++

		// Count by phase
		switch pod.Status.Phase {
		case corev1.PodRunning:
			status.Running++
		case corev1.PodPending:
			status.Pending++
		case corev1.PodFailed:
			status.Failed++
		case corev1.PodSucceeded:
			status.Succeeded++
		case corev1.PodUnknown:
			status.Unknown++
		}

		// Check for problem pods
		if isProblemPod(&pod) {
			problem := analyzePodProblem(&pod)
			status.ProblemPods = append(status.ProblemPods, problem)
		}
	}

	return status, nil
}

// isProblemPod checks if a pod has issues
func isProblemPod(pod *corev1.Pod) bool {
	// Failed pods
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}

	// Pending pods for too long
	if pod.Status.Phase == corev1.PodPending {
		return true
	}

	// Check container statuses for issues
	for _, cs := range pod.Status.ContainerStatuses {
		// High restart count
		if cs.RestartCount > 5 {
			return true
		}

		// Waiting state with specific reasons
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason == "CrashLoopBackOff" ||
				reason == "ImagePullBackOff" ||
				reason == "ErrImagePull" ||
				reason == "CreateContainerError" ||
				reason == "RunContainerError" {
				return true
			}
		}

		// Terminated state
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return true
		}
	}

	return false
}

// analyzePodProblem analyzes a problem pod and returns details
func analyzePodProblem(pod *corev1.Pod) ProblemPod {
	problem := ProblemPod{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    string(pod.Status.Phase),
	}

	// Check container statuses
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			problem.Reason = cs.State.Waiting.Reason
			problem.Message = cs.State.Waiting.Message
			problem.Restarts = cs.RestartCount
			break
		}
		if cs.State.Terminated != nil {
			problem.Reason = cs.State.Terminated.Reason
			problem.Message = cs.State.Terminated.Message
			problem.Restarts = cs.RestartCount
			break
		}
		if cs.RestartCount > 5 {
			problem.Reason = fmt.Sprintf("HighRestartCount(%d)", cs.RestartCount)
			problem.Restarts = cs.RestartCount
		}
	}

	// Check pod conditions
	if problem.Reason == "" {
		for _, condition := range pod.Status.Conditions {
			if condition.Status == corev1.ConditionFalse {
				problem.Reason = condition.Reason
				problem.Message = condition.Message
				break
			}
		}
	}

	// If still no reason, use phase
	if problem.Reason == "" {
		problem.Reason = string(pod.Status.Phase)
		problem.Message = pod.Status.Message
	}

	return problem
}
