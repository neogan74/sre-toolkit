package healthcheck

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestAuditPodProbes(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		wantNil        bool
		wantContainers int
		wantIssues     int // total issues across all containers
	}{
		{
			name: "pod with all probes configured",
			pod: makePodWithProbes("pod1", "default", []containerProbeConfig{
				{name: "container1", hasLiveness: true, hasReadiness: true},
			}),
			wantNil: true,
		},
		{
			name: "pod missing liveness probe",
			pod: makePodWithProbes("pod1", "default", []containerProbeConfig{
				{name: "container1", hasLiveness: false, hasReadiness: true},
			}),
			wantNil:        false,
			wantContainers: 1,
			wantIssues:     1,
		},
		{
			name: "pod missing readiness probe",
			pod: makePodWithProbes("pod1", "default", []containerProbeConfig{
				{name: "container1", hasLiveness: true, hasReadiness: false},
			}),
			wantNil:        false,
			wantContainers: 1,
			wantIssues:     1,
		},
		{
			name: "pod missing both probes",
			pod: makePodWithProbes("pod1", "default", []containerProbeConfig{
				{name: "container1", hasLiveness: false, hasReadiness: false},
			}),
			wantNil:        false,
			wantContainers: 1,
			wantIssues:     2,
		},
		{
			name: "multi-container pod with mixed probe configs",
			pod: makePodWithProbes("pod1", "default", []containerProbeConfig{
				{name: "container1", hasLiveness: true, hasReadiness: true},
				{name: "container2", hasLiveness: false, hasReadiness: false},
				{name: "container3", hasLiveness: true, hasReadiness: false},
			}),
			wantNil:        false,
			wantContainers: 2, // container2 and container3 have issues
			wantIssues:     3, // container2: 2 issues, container3: 1 issue
		},
		{
			name: "all containers have both probes",
			pod: makePodWithProbes("pod1", "default", []containerProbeConfig{
				{name: "container1", hasLiveness: true, hasReadiness: true},
				{name: "container2", hasLiveness: true, hasReadiness: true},
			}),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditPodProbes(tt.pod)

			if tt.wantNil {
				if got != nil {
					t.Errorf("AuditPodProbes() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("AuditPodProbes() = nil, want non-nil")
				return
			}

			if got.Pod != tt.pod.Name {
				t.Errorf("AuditPodProbes().Pod = %v, want %v", got.Pod, tt.pod.Name)
			}

			if got.Namespace != tt.pod.Namespace {
				t.Errorf("AuditPodProbes().Namespace = %v, want %v", got.Namespace, tt.pod.Namespace)
			}

			if len(got.Containers) != tt.wantContainers {
				t.Errorf("AuditPodProbes().Containers count = %v, want %v", len(got.Containers), tt.wantContainers)
			}

			// Count total issues
			totalIssues := 0
			for _, c := range got.Containers {
				totalIssues += len(c.Issues)
			}
			if totalIssues != tt.wantIssues {
				t.Errorf("AuditPodProbes() total issues = %v, want %v", totalIssues, tt.wantIssues)
			}
		})
	}
}

func TestAuditPodProbes_IssueMessages(t *testing.T) {
	pod := makePodWithProbes("test-pod", "test-ns", []containerProbeConfig{
		{name: "main", hasLiveness: false, hasReadiness: false},
	})

	got := AuditPodProbes(pod)

	if got == nil {
		t.Fatal("AuditPodProbes() = nil, want non-nil")
	}

	if len(got.Containers) != 1 {
		t.Fatalf("expected 1 container issue, got %d", len(got.Containers))
	}

	container := got.Containers[0]
	if container.Name != "main" {
		t.Errorf("container name = %v, want main", container.Name)
	}

	// Check for specific issue messages
	issues := container.Issues
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}

	foundLiveness := false
	foundReadiness := false
	for _, issue := range issues {
		if issue == "Liveness probe not configured" {
			foundLiveness = true
		}
		if issue == "Readiness probe not configured" {
			foundReadiness = true
		}
	}

	if !foundLiveness {
		t.Error("missing 'Liveness probe not configured' issue")
	}
	if !foundReadiness {
		t.Error("missing 'Readiness probe not configured' issue")
	}
}

// Helper types and functions

type containerProbeConfig struct {
	name         string
	hasLiveness  bool
	hasReadiness bool
}

func makePodWithProbes(name, namespace string, containers []containerProbeConfig) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
	}

	for _, c := range containers {
		container := corev1.Container{
			Name:  c.name,
			Image: "nginx:latest",
		}

		if c.hasLiveness {
			container.LivenessProbe = &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/healthz",
						Port: intstr.FromInt(8080),
					},
				},
			}
		}

		if c.hasReadiness {
			container.ReadinessProbe = &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/ready",
						Port: intstr.FromInt(8080),
					},
				},
			}
		}

		pod.Spec.Containers = append(pod.Spec.Containers, container)
	}

	return pod
}
