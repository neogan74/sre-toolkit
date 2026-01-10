package healthcheck

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckPods(t *testing.T) {
	tests := []struct {
		name             string
		pods             []corev1.Pod
		namespace        string
		wantTotal        int
		wantRunning      int
		wantPending      int
		wantFailed       int
		wantProblemCount int
		wantErr          bool
	}{
		{
			name: "all pods running",
			pods: []corev1.Pod{
				makePod("pod1", "default", podOptions{phase: corev1.PodRunning}),
				makePod("pod2", "default", podOptions{phase: corev1.PodRunning}),
			},
			namespace:        "",
			wantTotal:        2,
			wantRunning:      2,
			wantProblemCount: 0,
			wantErr:          false,
		},
		{
			name: "pod with CrashLoopBackOff",
			pods: []corev1.Pod{
				makePod("pod1", "default", podOptions{phase: corev1.PodRunning}),
				makePod("pod2", "default", podOptions{
					phase: corev1.PodRunning,
					containerState: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "Back-off restarting failed container",
						},
					},
					restarts: 10,
				}),
			},
			namespace:        "",
			wantTotal:        2,
			wantRunning:      2,
			wantProblemCount: 1, // CrashLoopBackOff is a problem
			wantErr:          false,
		},
		{
			name: "pod with ImagePullBackOff",
			pods: []corev1.Pod{
				makePod("pod1", "default", podOptions{
					phase: corev1.PodPending,
					containerState: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "ImagePullBackOff",
							Message: "Back-off pulling image",
						},
					},
				}),
			},
			namespace:        "",
			wantTotal:        1,
			wantPending:      1,
			wantProblemCount: 1, // ImagePullBackOff + Pending = problem
			wantErr:          false,
		},
		{
			name: "failed pods",
			pods: []corev1.Pod{
				makePod("pod1", "default", podOptions{phase: corev1.PodFailed}),
			},
			namespace:        "",
			wantTotal:        1,
			wantFailed:       1,
			wantProblemCount: 1,
			wantErr:          false,
		},
		{
			name: "high restart count",
			pods: []corev1.Pod{
				makePod("pod1", "default", podOptions{
					phase:    corev1.PodRunning,
					restarts: 15,
				}),
			},
			namespace:        "",
			wantTotal:        1,
			wantRunning:      1,
			wantProblemCount: 1, // High restarts = problem
			wantErr:          false,
		},
		{
			name: "namespace filtering",
			pods: []corev1.Pod{
				makePod("pod1", "default", podOptions{phase: corev1.PodRunning}),
				makePod("pod2", "kube-system", podOptions{phase: corev1.PodRunning}),
			},
			namespace:        "default",
			wantTotal:        1, // Only default namespace
			wantRunning:      1,
			wantProblemCount: 0,
			wantErr:          false,
		},
		{
			name:             "no pods",
			pods:             []corev1.Pod{},
			namespace:        "",
			wantTotal:        0,
			wantProblemCount: 0,
			wantErr:          false,
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

			// Run CheckPods
			got, err := CheckPods(context.Background(), clientset, tt.namespace)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPods() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check totals
			if got.Total != tt.wantTotal {
				t.Errorf("CheckPods().Total = %v, want %v", got.Total, tt.wantTotal)
			}
			if got.Running != tt.wantRunning {
				t.Errorf("CheckPods().Running = %v, want %v", got.Running, tt.wantRunning)
			}
			if got.Pending != tt.wantPending {
				t.Errorf("CheckPods().Pending = %v, want %v", got.Pending, tt.wantPending)
			}
			if got.Failed != tt.wantFailed {
				t.Errorf("CheckPods().Failed = %v, want %v", got.Failed, tt.wantFailed)
			}
			if len(got.ProblemPods) != tt.wantProblemCount {
				t.Errorf("CheckPods().ProblemPods count = %v, want %v", len(got.ProblemPods), tt.wantProblemCount)
			}
		})
	}
}

func TestIsProblemPod(t *testing.T) {
	tests := []struct {
		name   string
		pod    *corev1.Pod
		want   bool
		reason string
	}{
		{
			name: "healthy running pod",
			pod:  makePodPtr("pod1", "default", podOptions{phase: corev1.PodRunning}),
			want: false,
		},
		{
			name:   "failed pod",
			pod:    makePodPtr("pod1", "default", podOptions{phase: corev1.PodFailed}),
			want:   true,
			reason: "Pod failed",
		},
		{
			name:   "pending pod",
			pod:    makePodPtr("pod1", "default", podOptions{phase: corev1.PodPending}),
			want:   true,
			reason: "Pod pending",
		},
		{
			name: "CrashLoopBackOff",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodRunning,
				containerState: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason: "CrashLoopBackOff",
					},
				},
			}),
			want:   true,
			reason: "CrashLoopBackOff",
		},
		{
			name: "ImagePullBackOff",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodPending,
				containerState: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason: "ImagePullBackOff",
					},
				},
			}),
			want:   true,
			reason: "ImagePullBackOff",
		},
		{
			name: "high restart count",
			pod: makePodPtr("pod1", "default", podOptions{
				phase:    corev1.PodRunning,
				restarts: 10,
			}),
			want:   true,
			reason: "High restarts",
		},
		{
			name: "medium restart count (< 6)",
			pod: makePodPtr("pod1", "default", podOptions{
				phase:    corev1.PodRunning,
				restarts: 3,
			}),
			want: false,
		},
		{
			name: "terminated with non-zero exit code",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodFailed,
				containerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
					},
				},
			}),
			want:   true,
			reason: "Failed + terminated",
		},
		{
			name: "CreateContainerError",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodPending,
				containerState: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason: "CreateContainerError",
					},
				},
			}),
			want:   true,
			reason: "CreateContainerError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProblemPod(tt.pod)
			if got != tt.want {
				t.Errorf("isProblemPod() = %v, want %v (reason: %s)", got, tt.want, tt.reason)
			}
		})
	}
}

func TestAnalyzePodProblem(t *testing.T) {
	tests := []struct {
		name       string
		pod        *corev1.Pod
		wantReason string
		wantStatus string
	}{
		{
			name: "CrashLoopBackOff",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodRunning,
				containerState: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CrashLoopBackOff",
						Message: "Back-off restarting failed container",
					},
				},
				restarts: 10,
			}),
			wantReason: "CrashLoopBackOff",
			wantStatus: "Running",
		},
		{
			name: "ImagePullBackOff",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodPending,
				containerState: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ImagePullBackOff",
						Message: "image not found",
					},
				},
			}),
			wantReason: "ImagePullBackOff",
			wantStatus: "Pending",
		},
		{
			name: "high restart count",
			pod: makePodPtr("pod1", "default", podOptions{
				phase:    corev1.PodRunning,
				restarts: 15,
			}),
			wantReason: "HighRestartCount(15)",
			wantStatus: "Running",
		},
		{
			name: "terminated",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodFailed,
				containerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason:   "Error",
						Message:  "Container exited with error",
						ExitCode: 1,
					},
				},
			}),
			wantReason: "Error",
			wantStatus: "Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzePodProblem(tt.pod)

			if got.Name != tt.pod.Name {
				t.Errorf("analyzePodProblem().Name = %v, want %v", got.Name, tt.pod.Name)
			}
			if got.Namespace != tt.pod.Namespace {
				t.Errorf("analyzePodProblem().Namespace = %v, want %v", got.Namespace, tt.pod.Namespace)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("analyzePodProblem().Status = %v, want %v", got.Status, tt.wantStatus)
			}
			if got.Reason != tt.wantReason {
				t.Errorf("analyzePodProblem().Reason = %v, want %v", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestAuditPodResources(t *testing.T) {
	tests := []struct {
		name       string
		pod        *corev1.Pod
		wantIssues int
	}{
		{
			name: "pod with limits and requests",
			pod: makePodPtr("pod1", "default", podOptions{
				phase:  corev1.PodRunning,
				cpuReq: "100m",
				memReq: "128Mi",
				cpuLim: "200m",
				memLim: "256Mi",
			}),
			wantIssues: 0,
		},
		{
			name: "pod missing all limits and requests",
			pod: makePodPtr("pod1", "default", podOptions{
				phase: corev1.PodRunning,
			}),
			wantIssues: 1, // One ContainerResourceIssue
		},
		{
			name: "pod missing some limits",
			pod: makePodPtr("pod1", "default", podOptions{
				phase:  corev1.PodRunning,
				cpuReq: "100m",
				memReq: "128Mi",
			}),
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := auditPodResources(tt.pod)
			if tt.wantIssues == 0 {
				if got != nil {
					t.Errorf("auditPodResources() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("auditPodResources() = nil, want %d issues", tt.wantIssues)
				} else if len(got.Containers) != tt.wantIssues {
					t.Errorf("auditPodResources() issues count = %d, want %d", len(got.Containers), tt.wantIssues)
				}
			}
		})
	}
}

// Helper functions for test data

type podOptions struct {
	phase          corev1.PodPhase
	containerState corev1.ContainerState
	restarts       int32
	cpuReq         string
	memReq         string
	cpuLim         string
	memLim         string
}

func makePod(name, namespace string, opts podOptions) corev1.Pod {
	return *makePodPtr(name, namespace, opts)
}

func makePodPtr(name, namespace string, opts podOptions) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase:             opts.phase,
			ContainerStatuses: []corev1.ContainerStatus{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
	}

	// Add container resources if specified
	resources := corev1.ResourceRequirements{
		Requests: make(corev1.ResourceList),
		Limits:   make(corev1.ResourceList),
	}
	if opts.cpuReq != "" {
		resources.Requests[corev1.ResourceCPU] = resource.MustParse(opts.cpuReq)
	}
	if opts.memReq != "" {
		resources.Requests[corev1.ResourceMemory] = resource.MustParse(opts.memReq)
	}
	if opts.cpuLim != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(opts.cpuLim)
	}
	if opts.memLim != "" {
		resources.Limits[corev1.ResourceMemory] = resource.MustParse(opts.memLim)
	}

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:      "container",
		Resources: resources,
	})

	// Add container status if state is specified
	if opts.containerState.Waiting != nil || opts.containerState.Running != nil || opts.containerState.Terminated != nil {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:         "container",
			State:        opts.containerState,
			RestartCount: opts.restarts,
			Ready:        false,
		})
	} else if opts.restarts > 0 {
		// Just add restart count
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:         "container",
			RestartCount: opts.restarts,
			Ready:        true,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{},
			},
		})
	} else if opts.phase == corev1.PodRunning {
		// Running pod with healthy container
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:         "container",
			RestartCount: 0,
			Ready:        true,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{},
			},
		})
	}

	return pod
}
