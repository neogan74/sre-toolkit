package audit

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRunAudit(t *testing.T) {
	tests := []struct {
		name                string
		namespace           string
		objects             []runtime.Object
		wantResources       int
		wantProbes          int
		wantSecurity        int
		wantNetworkPolicies int
		wantCritical        int
		wantWarning         int
	}{
		{
			name: "healthy namespace",
			objects: []runtime.Object{
				makeNamespace("default"),
				makeHealthyPod("app", "default"),
			},
			wantResources:       0,
			wantProbes:          0,
			wantSecurity:        0,
			wantNetworkPolicies: 1,
			wantCritical:        0,
			wantWarning:         1,
		},
		{
			name:      "namespace filter limits scope",
			namespace: "default",
			objects: []runtime.Object{
				makeNamespace("default"),
				makeNamespace("other"),
				makeBrokenPod("app", "default"),
				makeBrokenPod("app", "other"),
			},
			wantResources:       4,
			wantProbes:          2,
			wantSecurity:        2,
			wantNetworkPolicies: 1,
			wantCritical:        2,
			wantWarning:         7,
		},
		{
			name: "network policy suppresses warning",
			objects: []runtime.Object{
				makeNamespace("secure"),
				makeHealthyPod("app", "secure"),
				makeNetworkPolicy("default-deny", "secure"),
			},
			wantResources:       0,
			wantProbes:          0,
			wantSecurity:        0,
			wantNetworkPolicies: 0,
			wantCritical:        0,
			wantWarning:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.objects...)

			got, err := RunAudit(context.Background(), clientset, tt.namespace)
			if err != nil {
				t.Fatalf("RunAudit() error = %v", err)
			}

			if len(got.ResourceIssues) != tt.wantResources {
				t.Fatalf("RunAudit() resource issues = %d, want %d", len(got.ResourceIssues), tt.wantResources)
			}
			if len(got.ProbeIssues) != tt.wantProbes {
				t.Fatalf("RunAudit() probe issues = %d, want %d", len(got.ProbeIssues), tt.wantProbes)
			}
			if len(got.SecurityIssues) != tt.wantSecurity {
				t.Fatalf("RunAudit() security issues = %d, want %d", len(got.SecurityIssues), tt.wantSecurity)
			}
			if len(got.NetworkPolicyIssues) != tt.wantNetworkPolicies {
				t.Fatalf("RunAudit() network policy issues = %d, want %d", len(got.NetworkPolicyIssues), tt.wantNetworkPolicies)
			}
			if got.Summary.CriticalCount != tt.wantCritical {
				t.Fatalf("RunAudit() critical count = %d, want %d", got.Summary.CriticalCount, tt.wantCritical)
			}
			if got.Summary.WarningCount != tt.wantWarning {
				t.Fatalf("RunAudit() warning count = %d, want %d", got.Summary.WarningCount, tt.wantWarning)
			}
		})
	}
}

func makeNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func makeNetworkPolicy(name, namespace string) runtime.Object {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func makeHealthyPod(name, namespace string) *corev1.Pod {
	runAsNonRoot := true
	readOnlyRootFilesystem := true

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					LivenessProbe:  &corev1.Probe{},
					ReadinessProbe: &corev1.Probe{},
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
					},
				},
			},
		},
	}
}

func makeBrokenPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
				},
			},
		},
	}
}
