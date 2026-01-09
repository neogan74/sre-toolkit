package diagnostics

import (
	"context"
	"testing"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRunDiagnostics(t *testing.T) {
	tests := []struct {
		name             string
		nodes            []corev1.Node
		pods             []corev1.Pod
		namespace        string
		wantNodeIssues   int
		wantPodIssues    int
		wantSystemIssues int
		wantCritical     int
		wantWarning      int
		wantErr          bool
	}{
		{
			name: "healthy cluster",
			nodes: []corev1.Node{
				makeHealthyNode("node1"),
			},
			pods: []corev1.Pod{
				makeHealthyPod("pod1", "default"),
			},
			namespace:        "",
			wantNodeIssues:   0,
			wantPodIssues:    0,
			wantSystemIssues: 0,
			wantCritical:     0,
			wantWarning:      0,
			wantErr:          false,
		},
		{
			name: "cluster with node issues",
			nodes: []corev1.Node{
				makeNotReadyNode("node1"),
				makeNodeWithMemoryPressure("node2"),
			},
			pods:             []corev1.Pod{},
			namespace:        "",
			wantNodeIssues:   3, // NotReady status + NotReady issue + MemoryPressure issue
			wantPodIssues:    0,
			wantSystemIssues: 0,
			wantCritical:     2, // NotReady status + MemoryPressure
			wantWarning:      1, // NotReady issue message
			wantErr:          false,
		},
		{
			name:  "cluster with pod issues",
			nodes: []corev1.Node{},
			pods: []corev1.Pod{
				makeCrashLoopPod("pod1", "default"),
				makeImagePullBackOffPod("pod2", "default"),
				makeHighRestartPod("pod3", "default", 15),
			},
			namespace:        "",
			wantNodeIssues:   0,
			wantPodIssues:    3,
			wantSystemIssues: 0,
			wantCritical:     3, // All critical
			wantWarning:      0,
			wantErr:          false,
		},
		{
			name: "mixed severity issues",
			nodes: []corev1.Node{
				makeCordonedNode("node1"),
			},
			pods: []corev1.Pod{
				makeHighRestartPod("pod1", "default", 7), // Warning (5-10 restarts)
			},
			namespace:        "",
			wantNodeIssues:   1, // Cordoned (Info)
			wantPodIssues:    1, // Moderate restarts (Warning)
			wantSystemIssues: 0,
			wantCritical:     0,
			wantWarning:      1, // Moderate restarts
			// Note: Cordoned node is Info severity, not counted in warning
			wantErr: false,
		},
		{
			name: "namespace filtering",
			nodes: []corev1.Node{
				makeHealthyNode("node1"),
			},
			pods: []corev1.Pod{
				makeCrashLoopPod("pod1", "default"),
				makeCrashLoopPod("pod2", "kube-system"),
			},
			namespace:        "default",
			wantNodeIssues:   0,
			wantPodIssues:    1, // Only default namespace
			wantSystemIssues: 0,
			wantCritical:     1,
			wantWarning:      0,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset
			var objs []runtime.Object
			for i := range tt.nodes {
				objs = append(objs, &tt.nodes[i])
			}
			for i := range tt.pods {
				objs = append(objs, &tt.pods[i])
			}
			clientset := fake.NewSimpleClientset(objs...)

			// Run diagnostics
			got, err := RunDiagnostics(context.Background(), clientset, tt.namespace)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("RunDiagnostics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Check issue counts
			if len(got.NodeIssues) != tt.wantNodeIssues {
				t.Errorf("RunDiagnostics() NodeIssues = %d, want %d", len(got.NodeIssues), tt.wantNodeIssues)
			}
			if len(got.PodIssues) != tt.wantPodIssues {
				t.Errorf("RunDiagnostics() PodIssues = %d, want %d", len(got.PodIssues), tt.wantPodIssues)
			}
			if len(got.SystemIssues) != tt.wantSystemIssues {
				t.Errorf("RunDiagnostics() SystemIssues = %d, want %d", len(got.SystemIssues), tt.wantSystemIssues)
			}

			// Check severity counts
			if got.Summary.CriticalCount != tt.wantCritical {
				t.Errorf("RunDiagnostics() CriticalCount = %d, want %d", got.Summary.CriticalCount, tt.wantCritical)
			}
			if got.Summary.WarningCount != tt.wantWarning {
				t.Errorf("RunDiagnostics() WarningCount = %d, want %d", got.Summary.WarningCount, tt.wantWarning)
			}
		})
	}
}

func TestDiagnoseNode(t *testing.T) {
	tests := []struct {
		name         string
		nodeStatus   healthcheck.NodeStatus
		wantIssues   int
		wantCritical int
		wantWarning  int
		wantInfo     int
	}{
		{
			name: "healthy node",
			nodeStatus: healthcheck.NodeStatus{
				Name:   "node1",
				Status: "Ready",
				Issues: []string{},
			},
			wantIssues:   0,
			wantCritical: 0,
			wantWarning:  0,
			wantInfo:     0,
		},
		{
			name: "not ready node",
			nodeStatus: healthcheck.NodeStatus{
				Name:   "node1",
				Status: "NotReady",
				Issues: []string{},
			},
			wantIssues:   1,
			wantCritical: 1,
			wantWarning:  0,
			wantInfo:     0,
		},
		{
			name: "memory pressure",
			nodeStatus: healthcheck.NodeStatus{
				Name:   "node1",
				Status: "Ready",
				Issues: []string{"Memory pressure detected"},
			},
			wantIssues:   1,
			wantCritical: 1,
			wantWarning:  0,
			wantInfo:     0,
		},
		{
			name: "disk pressure",
			nodeStatus: healthcheck.NodeStatus{
				Name:   "node1",
				Status: "Ready",
				Issues: []string{"Disk pressure detected"},
			},
			wantIssues:   1,
			wantCritical: 1,
			wantWarning:  0,
			wantInfo:     0,
		},
		{
			name: "cordoned node",
			nodeStatus: healthcheck.NodeStatus{
				Name:   "node1",
				Status: "Ready",
				Issues: []string{"Node is cordoned (unschedulable)"},
			},
			wantIssues:   1,
			wantCritical: 0,
			wantWarning:  0,
			wantInfo:     1,
		},
		{
			name: "multiple issues",
			nodeStatus: healthcheck.NodeStatus{
				Name:   "node1",
				Status: "NotReady",
				Issues: []string{
					"Memory pressure detected",
					"Disk pressure detected",
					"Node is cordoned (unschedulable)",
				},
			},
			wantIssues:   4, // NotReady + 3 issues
			wantCritical: 3, // NotReady + memory + disk
			wantWarning:  0,
			wantInfo:     1, // Cordoned
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnoseNode(&tt.nodeStatus)

			if len(got) != tt.wantIssues {
				t.Errorf("diagnoseNode() returned %d issues, want %d", len(got), tt.wantIssues)
			}

			critical := 0
			warning := 0
			info := 0
			for _, issue := range got {
				switch issue.Severity {
				case "Critical":
					critical++
				case "Warning":
					warning++
				case "Info":
					info++
				}
			}

			if critical != tt.wantCritical {
				t.Errorf("diagnoseNode() critical = %d, want %d", critical, tt.wantCritical)
			}
			if warning != tt.wantWarning {
				t.Errorf("diagnoseNode() warning = %d, want %d", warning, tt.wantWarning)
			}
			if info != tt.wantInfo {
				t.Errorf("diagnoseNode() info = %d, want %d", info, tt.wantInfo)
			}
		})
	}
}

func TestDiagnosePod(t *testing.T) {
	tests := []struct {
		name         string
		problemPod   healthcheck.ProblemPod
		wantSeverity string
		wantType     string
	}{
		{
			name: "CrashLoopBackOff",
			problemPod: healthcheck.ProblemPod{
				Name:      "pod1",
				Namespace: "default",
				Reason:    "CrashLoopBackOff",
				Restarts:  10,
			},
			wantSeverity: "Critical",
			wantType:     "CrashLoopBackOff",
		},
		{
			name: "ImagePullBackOff",
			problemPod: healthcheck.ProblemPod{
				Name:      "pod1",
				Namespace: "default",
				Reason:    "ImagePullBackOff",
			},
			wantSeverity: "Critical",
			wantType:     "ImagePullError",
		},
		{
			name: "high restart count",
			problemPod: healthcheck.ProblemPod{
				Name:      "pod1",
				Namespace: "default",
				Restarts:  15,
			},
			wantSeverity: "Critical",
			wantType:     "HighRestartCount",
		},
		{
			name: "moderate restart count",
			problemPod: healthcheck.ProblemPod{
				Name:      "pod1",
				Namespace: "default",
				Restarts:  7,
			},
			wantSeverity: "Warning",
			wantType:     "FrequentRestarts",
		},
		{
			name: "pending pod",
			problemPod: healthcheck.ProblemPod{
				Name:      "pod1",
				Namespace: "default",
				Reason:    "Pending",
			},
			wantSeverity: "Warning",
			wantType:     "PodPending",
		},
		{
			name: "failed pod",
			problemPod: healthcheck.ProblemPod{
				Name:      "pod1",
				Namespace: "default",
				Reason:    "Failed",
			},
			wantSeverity: "Critical",
			wantType:     "PodFailed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnosePod(&tt.problemPod)

			if got.Severity != tt.wantSeverity {
				t.Errorf("diagnosePod() Severity = %v, want %v", got.Severity, tt.wantSeverity)
			}
			if got.Type != tt.wantType {
				t.Errorf("diagnosePod() Type = %v, want %v", got.Type, tt.wantType)
			}
			if got.Pod != tt.problemPod.Name {
				t.Errorf("diagnosePod() Pod = %v, want %v", got.Pod, tt.problemPod.Name)
			}
			if got.Namespace != tt.problemPod.Namespace {
				t.Errorf("diagnosePod() Namespace = %v, want %v", got.Namespace, tt.problemPod.Namespace)
			}
		})
	}
}

func TestDiagnoseComponent(t *testing.T) {
	tests := []struct {
		name      string
		component healthcheck.ComponentStatus
		wantIssue bool
		wantType  string
	}{
		{
			name: "healthy component",
			component: healthcheck.ComponentStatus{
				Name:   "kube-apiserver",
				Status: "Healthy",
			},
			wantIssue: false,
		},
		{
			name: "unhealthy component",
			component: healthcheck.ComponentStatus{
				Name:    "etcd",
				Status:  "Unhealthy",
				Message: "Connection refused",
			},
			wantIssue: true,
			wantType:  "ComponentUnhealthy",
		},
		{
			name: "unknown component status",
			component: healthcheck.ComponentStatus{
				Name:   "kube-scheduler",
				Status: "Unknown",
			},
			wantIssue: true,
			wantType:  "ComponentUnhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnoseComponent(&tt.component)

			if tt.wantIssue {
				if got == nil {
					t.Errorf("diagnoseComponent() returned nil, want issue")
					return
				}
				if got.Component != tt.component.Name {
					t.Errorf("diagnoseComponent() Component = %v, want %v", got.Component, tt.component.Name)
				}
				if got.Severity != "Critical" {
					t.Errorf("diagnoseComponent() Severity = %v, want Critical", got.Severity)
				}
				if got.Type != tt.wantType {
					t.Errorf("diagnoseComponent() Type = %v, want %v", got.Type, tt.wantType)
				}
			} else {
				if got != nil {
					t.Errorf("diagnoseComponent() returned issue for healthy component")
				}
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	tests := []struct {
		name         string
		result       *Result
		wantTotal    int
		wantCritical int
		wantWarning  int
		wantInfo     int
	}{
		{
			name: "no issues",
			result: &Result{
				NodeIssues:   []NodeIssue{},
				PodIssues:    []PodIssue{},
				SystemIssues: []SystemIssue{},
			},
			wantTotal:    0,
			wantCritical: 0,
			wantWarning:  0,
			wantInfo:     0,
		},
		{
			name: "mixed issues",
			result: &Result{
				NodeIssues: []NodeIssue{
					{Severity: "Critical"},
					{Severity: "Warning"},
					{Severity: "Info"},
				},
				PodIssues: []PodIssue{
					{Severity: "Critical"},
					{Severity: "Critical"},
				},
				SystemIssues: []SystemIssue{
					{Severity: "Critical"},
				},
			},
			wantTotal:    6,
			wantCritical: 4,
			wantWarning:  1,
			wantInfo:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSummary(tt.result)

			if got.TotalIssues != tt.wantTotal {
				t.Errorf("calculateSummary() TotalIssues = %v, want %v", got.TotalIssues, tt.wantTotal)
			}
			if got.CriticalCount != tt.wantCritical {
				t.Errorf("calculateSummary() CriticalCount = %v, want %v", got.CriticalCount, tt.wantCritical)
			}
			if got.WarningCount != tt.wantWarning {
				t.Errorf("calculateSummary() WarningCount = %v, want %v", got.WarningCount, tt.wantWarning)
			}
			if got.InfoCount != tt.wantInfo {
				t.Errorf("calculateSummary() InfoCount = %v, want %v", got.InfoCount, tt.wantInfo)
			}
		})
	}
}

// Helper functions for test data

func makeHealthyNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func makeNotReadyNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:    corev1.NodeReady,
					Status:  corev1.ConditionFalse,
					Message: "kubelet is not ready",
				},
			},
		},
	}
}

func makeNodeWithMemoryPressure(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   corev1.NodeMemoryPressure,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func makeCordonedNode(name string) corev1.Node {
	node := makeHealthyNode(name)
	node.Spec.Unschedulable = true
	return node
}

func makeHealthyPod(name, namespace string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}

func makeCrashLoopPod(name, namespace string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready:        false,
					RestartCount: 10,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CrashLoopBackOff",
						},
					},
				},
			},
		},
	}
}

func makeImagePullBackOffPod(name, namespace string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "ImagePullBackOff",
						},
					},
				},
			},
		},
	}
}

func makeHighRestartPod(name, namespace string, restarts int32) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready:        true,
					RestartCount: restarts,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}
