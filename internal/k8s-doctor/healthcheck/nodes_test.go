package healthcheck

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckNodes(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []corev1.Node
		wantCount int
		wantErr   bool
	}{
		{
			name: "single healthy node",
			nodes: []corev1.Node{
				makeNode("node1", nodeOptions{
					ready:   true,
					version: "v1.28.0",
					roles:   []string{"node-role.kubernetes.io/control-plane"},
				}),
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple nodes with issues",
			nodes: []corev1.Node{
				makeNode("node1", nodeOptions{ready: true}),
				makeNode("node2", nodeOptions{ready: false}),
				makeNode("node3", nodeOptions{ready: true, memoryPressure: true}),
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "no nodes",
			nodes:     []corev1.Node{},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with nodes
			var objs []runtime.Object
			for i := range tt.nodes {
				objs = append(objs, &tt.nodes[i])
			}
			clientset := fake.NewSimpleClientset(objs...)

			// Run CheckNodes
			got, err := CheckNodes(context.Background(), clientset)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check count
			if len(got) != tt.wantCount {
				t.Errorf("CheckNodes() returned %d nodes, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestAnalyzeNode(t *testing.T) {
	tests := []struct {
		name        string
		node        *corev1.Node
		wantStatus  string
		wantIssues  int
		wantRoles   int
		wantVersion string
	}{
		{
			name: "healthy control-plane node",
			node: makeNodePtr("control-plane-1", nodeOptions{
				ready:   true,
				version: "v1.28.0",
				roles:   []string{"node-role.kubernetes.io/control-plane"},
			}),
			wantStatus:  "Ready",
			wantIssues:  0,
			wantRoles:   1,
			wantVersion: "v1.28.0",
		},
		{
			name: "not ready node",
			node: makeNodePtr("worker-1", nodeOptions{
				ready:   false,
				version: "v1.27.0",
				roles:   []string{"node-role.kubernetes.io/worker"},
			}),
			wantStatus:  "NotReady",
			wantIssues:  1, // "Node is not ready" message
			wantRoles:   1,
			wantVersion: "v1.27.0",
		},
		{
			name: "node with memory pressure",
			node: makeNodePtr("worker-2", nodeOptions{
				ready:          true,
				memoryPressure: true,
				roles:          []string{"node-role.kubernetes.io/worker"},
			}),
			wantStatus:  "Ready",
			wantIssues:  1, // "Memory pressure detected"
			wantRoles:   1,
			wantVersion: "",
		},
		{
			name: "node with disk pressure",
			node: makeNodePtr("worker-3", nodeOptions{
				ready:        true,
				diskPressure: true,
			}),
			wantStatus: "Ready",
			wantIssues: 1, // "Disk pressure detected"
			wantRoles:  1, // Default to worker
		},
		{
			name: "node with PID pressure",
			node: makeNodePtr("worker-4", nodeOptions{
				ready:       true,
				pidPressure: true,
			}),
			wantStatus: "Ready",
			wantIssues: 1, // "PID pressure detected"
			wantRoles:  1,
		},
		{
			name: "cordoned node",
			node: makeNodePtr("worker-5", nodeOptions{
				ready:         true,
				unschedulable: true,
			}),
			wantStatus: "Ready",
			wantIssues: 1, // "Node is cordoned"
			wantRoles:  1,
		},
		{
			name: "node with multiple issues",
			node: makeNodePtr("worker-6", nodeOptions{
				ready:          false,
				memoryPressure: true,
				diskPressure:   true,
				unschedulable:  true,
			}),
			wantStatus: "NotReady",
			wantIssues: 4, // NotReady + memory + disk + cordoned
			wantRoles:  1,
		},
		{
			name: "node with network unavailable",
			node: makeNodePtr("worker-7", nodeOptions{
				ready:              true,
				networkUnavailable: true,
			}),
			wantStatus: "Ready",
			wantIssues: 1, // "Network unavailable"
			wantRoles:  1,
		},
		{
			name: "node with version skew",
			node: makeNodePtr("worker-8", nodeOptions{
				ready:   true,
				version: "v1.24.0", // 4 minor versions behind v1.28.0
			}),
			wantStatus: "Ready",
			wantIssues: 1, // Version skew detected
			wantRoles:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzeNode(tt.node, "v1.28.0", corev1.ResourceList{})

			if got.Name != tt.node.Name {
				t.Errorf("analyzeNode().Name = %v, want %v", got.Name, tt.node.Name)
			}

			if got.Status != tt.wantStatus {
				t.Errorf("analyzeNode().Status = %v, want %v", got.Status, tt.wantStatus)
			}

			if len(got.Issues) != tt.wantIssues {
				t.Errorf("analyzeNode().Issues = %v (count: %d), want count: %d", got.Issues, len(got.Issues), tt.wantIssues)
			}

			if len(got.Roles) != tt.wantRoles {
				t.Errorf("analyzeNode().Roles = %v (count: %d), want count: %d", got.Roles, len(got.Roles), tt.wantRoles)
			}

			if tt.wantVersion != "" && got.Version != tt.wantVersion {
				t.Errorf("analyzeNode().Version = %v, want %v", got.Version, tt.wantVersion)
			}
		})
	}
}

func TestGetRoles(t *testing.T) {
	tests := []struct {
		name      string
		labels    map[string]string
		wantRoles []string
	}{
		{
			name: "control-plane role",
			labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
			wantRoles: []string{"control-plane"},
		},
		{
			name: "master role (legacy)",
			labels: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
			wantRoles: []string{"control-plane"},
		},
		{
			name: "worker role",
			labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			wantRoles: []string{"worker"},
		},
		{
			name:      "no role labels",
			labels:    map[string]string{},
			wantRoles: []string{"worker"}, // Default to worker
		},
		{
			name: "other labels",
			labels: map[string]string{
				"kubernetes.io/hostname":      "node1",
				"topology.kubernetes.io/zone": "us-east-1a",
			},
			wantRoles: []string{"worker"}, // Default to worker
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tt.labels,
				},
			}

			got := getRoles(node)

			if len(got) != len(tt.wantRoles) {
				t.Errorf("getRoles() = %v, want %v", got, tt.wantRoles)
				return
			}

			// Check if roles match (order doesn't matter)
			roleMap := make(map[string]bool)
			for _, role := range got {
				roleMap[role] = true
			}
			for _, wantRole := range tt.wantRoles {
				if !roleMap[wantRole] {
					t.Errorf("getRoles() missing role %v, got %v", wantRole, got)
				}
			}
		})
	}
}

// Helper functions for test data

type nodeOptions struct {
	ready              bool
	memoryPressure     bool
	diskPressure       bool
	pidPressure        bool
	networkUnavailable bool
	unschedulable      bool
	version            string
	roles              []string
}

func makeNode(name string, opts nodeOptions) corev1.Node {
	return *makeNodePtr(name, opts)
}

func makeNodePtr(name string, opts nodeOptions) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: make(map[string]string),
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion: opts.version,
			},
		},
		Spec: corev1.NodeSpec{
			Unschedulable: opts.unschedulable,
		},
	}

	// Add role labels
	for _, role := range opts.roles {
		node.Labels[role] = ""
	}

	// Add Ready condition
	readyStatus := corev1.ConditionTrue
	if !opts.ready {
		readyStatus = corev1.ConditionFalse
	}
	node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
		Type:    corev1.NodeReady,
		Status:  readyStatus,
		Reason:  "KubeletReady",
		Message: "kubelet is posting ready status",
	})

	// Add pressure conditions
	if opts.memoryPressure {
		node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
			Type:    corev1.NodeMemoryPressure,
			Status:  corev1.ConditionTrue,
			Reason:  "KubeletHasInsufficientMemory",
			Message: "kubelet has insufficient memory available",
		})
	}

	if opts.diskPressure {
		node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
			Type:    corev1.NodeDiskPressure,
			Status:  corev1.ConditionTrue,
			Reason:  "KubeletHasInsufficientDisk",
			Message: "kubelet has insufficient disk space available",
		})
	}

	if opts.pidPressure {
		node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
			Type:    corev1.NodePIDPressure,
			Status:  corev1.ConditionTrue,
			Reason:  "KubeletHasInsufficientPID",
			Message: "kubelet has insufficient PID available",
		})
	}

	if opts.networkUnavailable {
		node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
			Type:    corev1.NodeNetworkUnavailable,
			Status:  corev1.ConditionTrue,
			Reason:  "NetworkPluginNotReady",
			Message: "network plugin is not ready",
		})
	}

	return node
}
