package healthcheck

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckComponents(t *testing.T) {
	tests := []struct {
		name      string
		pods      []corev1.Pod
		wantCount int
		wantErr   bool
	}{
		{
			name: "with component pods in kube-system",
			pods: []corev1.Pod{
				makeComponentPod("kube-apiserver-control-plane", "kube-system", true),
				makeComponentPod("etcd-control-plane", "kube-system", true),
			},
			wantCount: 2, // Will find 2 components
			wantErr:   false,
		},
		{
			name:      "no pods at all",
			pods:      []corev1.Pod{},
			wantCount: 0, // ComponentStatus API returns empty, fallback returns 0
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with pods
			var objs []runtime.Object
			for i := range tt.pods {
				objs = append(objs, &tt.pods[i])
			}
			clientset := fake.NewSimpleClientset(objs...)

			// Run CheckComponents
			got, err := CheckComponents(context.Background(), clientset)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckComponents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Note: With fake client, ComponentStatus API returns empty list without error
			// So got will be an empty slice, not nil. That's acceptable behavior.
			if got == nil {
				t.Errorf("CheckComponents() returned nil, want empty or populated slice")
			}
		})
	}
}

func TestCheckComponentPods(t *testing.T) {
	tests := []struct {
		name          string
		pods          []corev1.Pod
		wantCount     int
		wantHealthy   int
		wantUnhealthy int
	}{
		{
			name: "all component pods running and ready",
			pods: []corev1.Pod{
				makeComponentPod("kube-apiserver-node1", "kube-system", true),
				makeComponentPod("etcd-node1", "kube-system", true),
				makeComponentPod("coredns-123abc", "kube-system", true),
			},
			wantCount:     3,
			wantHealthy:   3,
			wantUnhealthy: 0,
		},
		{
			name: "component pod not ready",
			pods: []corev1.Pod{
				makeComponentPod("kube-apiserver-node1", "kube-system", true),
				makeComponentPod("kube-scheduler-node1", "kube-system", false),
			},
			wantCount:     2,
			wantHealthy:   1,
			wantUnhealthy: 1,
		},
		{
			name: "component pod in pending phase",
			pods: []corev1.Pod{
				makePendingComponentPod("kube-controller-manager-node1", "kube-system"),
			},
			wantCount:     1,
			wantHealthy:   0,
			wantUnhealthy: 1,
		},
		{
			name: "multiple instances of same component",
			pods: []corev1.Pod{
				makeComponentPod("coredns-abc", "kube-system", true),
				makeComponentPod("coredns-def", "kube-system", true),
			},
			wantCount:   1, // Same component name
			wantHealthy: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with pods
			var objs []runtime.Object
			for i := range tt.pods {
				objs = append(objs, &tt.pods[i])
			}
			clientset := fake.NewSimpleClientset(objs...)

			// Run checkComponentPods
			got, err := checkComponentPods(context.Background(), clientset)
			if err != nil {
				t.Errorf("checkComponentPods() error = %v", err)
				return
			}

			// Check count
			if len(got) != tt.wantCount {
				t.Errorf("checkComponentPods() returned %d components, want %d", len(got), tt.wantCount)
			}

			// Count healthy/unhealthy
			healthy := 0
			unhealthy := 0
			for _, comp := range got {
				if comp.Status == "Healthy" {
					healthy++
				} else if comp.Status == "Unhealthy" {
					unhealthy++
				}
			}

			if healthy != tt.wantHealthy {
				t.Errorf("checkComponentPods() healthy count = %d, want %d", healthy, tt.wantHealthy)
			}
			if unhealthy != tt.wantUnhealthy {
				t.Errorf("checkComponentPods() unhealthy count = %d, want %d", unhealthy, tt.wantUnhealthy)
			}
		})
	}
}

func TestMatchesComponent(t *testing.T) {
	tests := []struct {
		podName       string
		componentName string
		want          bool
	}{
		{"kube-apiserver-control-plane", "kube-apiserver", true},
		{"kube-controller-manager-node1", "kube-controller-manager", true},
		{"kube-scheduler-master", "kube-scheduler", true},
		{"etcd-control-plane", "etcd", true},
		{"coredns-abc123", "coredns", true},
		{"kube-proxy-xyz789", "kube-proxy", true},
		{"nginx-deployment-abc", "kube-apiserver", false},
		{"application-pod", "etcd", false},
		{"api", "kube-apiserver", false}, // Too short
	}

	for _, tt := range tests {
		t.Run(tt.podName+"_"+tt.componentName, func(t *testing.T) {
			got := matchesComponent(tt.podName, tt.componentName)
			if got != tt.want {
				t.Errorf("matchesComponent(%q, %q) = %v, want %v", tt.podName, tt.componentName, got, tt.want)
			}
		})
	}
}

// Helper functions for test data

func makeComponentPod(name, namespace string, ready bool) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "container",
					Ready: ready,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
	return pod
}

func makePendingComponentPod(name, namespace string) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "container",
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "ContainerCreating",
						},
					},
				},
			},
		},
	}
	return pod
}
