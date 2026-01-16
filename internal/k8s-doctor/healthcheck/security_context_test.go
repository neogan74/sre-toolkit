package healthcheck

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAuditPodSecurityContext(t *testing.T) {
	runAsNonRootTrue := true
	runAsNonRootFalse := false
	readOnlyRootFilesystemTrue := true
	readOnlyRootFilesystemFalse := false

	tests := []struct {
		name       string
		pod        corev1.Pod
		wantIssues bool
		wantCount  int // Number of containers with issues
	}{
		{
			name: "secure pod",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "secure-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRootTrue,
					},
					Containers: []corev1.Container{
						{
							Name: "container1",
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: &readOnlyRootFilesystemTrue,
							},
						},
					},
				},
			},
			wantIssues: false,
		},
		{
			name: "insecure pod - no security context",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "insecure-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "container1"},
					},
				},
			},
			wantIssues: true,
			wantCount:  1,
		},
		{
			name: "insecure pod - runAsNonRoot false",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "root-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRootFalse,
					},
					Containers: []corev1.Container{
						{
							Name: "container1",
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: &readOnlyRootFilesystemTrue,
							},
						},
					},
				},
			},
			wantIssues: true,
			wantCount:  1,
		},
		{
			name: "insecure pod - readOnlyRootFilesystem false",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "writable-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRootTrue,
					},
					Containers: []corev1.Container{
						{
							Name: "container1",
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: &readOnlyRootFilesystemFalse,
							},
						},
					},
				},
			},
			wantIssues: true,
			wantCount:  1,
		},
		{
			name: "mixed containers",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "mixed-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRootTrue,
					},
					Containers: []corev1.Container{
						{
							Name: "secure-container",
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: &readOnlyRootFilesystemTrue,
							},
						},
						{
							Name: "insecure-container",
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: &readOnlyRootFilesystemFalse,
							},
						},
					},
				},
			},
			wantIssues: true,
			wantCount:  1,
		},
		{
			name: "container override runAsNonRoot",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "override-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRootFalse,
					},
					Containers: []corev1.Container{
						{
							Name: "secure-container",
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:           &runAsNonRootTrue,
								ReadOnlyRootFilesystem: &readOnlyRootFilesystemTrue,
							},
						},
					},
				},
			},
			wantIssues: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditPodSecurityContext(&tt.pod)

			if !tt.wantIssues {
				if got != nil {
					t.Errorf("AuditPodSecurityContext() returned issue %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("AuditPodSecurityContext() returned nil, want issue")
				return
			}

			if len(got.Containers) != tt.wantCount {
				t.Errorf("AuditPodSecurityContext() container issues = %d, want %d", len(got.Containers), tt.wantCount)
			}
		})
	}
}
