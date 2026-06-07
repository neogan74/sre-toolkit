package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNetworkPartition_Run(t *testing.T) {
	client := fake.NewSimpleClientset()

	cfg := NetworkPartitionConfig{
		Namespace:     "default",
		LabelSelector: "app=target",
		Duration:      100 * time.Millisecond,
		DryRun:        false,
	}

	partition := NewNetworkPartition(client, cfg)

	// Start the partition
	errChan := make(chan error)
	go func() {
		errChan <- partition.Run(context.Background())
	}()

	// Wait a tiny bit for the policy to be created
	time.Sleep(20 * time.Millisecond)

	// Verify the policy was created
	policies, err := client.NetworkingV1().NetworkPolicies("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, policies.Items, 1)

	policy := policies.Items[0]
	assert.Equal(t, "target", policy.Spec.PodSelector.MatchLabels["app"])
	assert.Len(t, policy.Spec.PolicyTypes, 2)

	// Wait for Run to complete
	err = <-errChan
	assert.NoError(t, err)

	// Verify the policy was deleted
	policies, err = client.NetworkingV1().NetworkPolicies("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, policies.Items, 0)
}

func TestNetworkPartition_DryRun(t *testing.T) {
	client := fake.NewSimpleClientset()

	cfg := NetworkPartitionConfig{
		Namespace:     "default",
		LabelSelector: "app=target",
		Duration:      100 * time.Millisecond,
		DryRun:        true,
	}

	partition := NewNetworkPartition(client, cfg)

	err := partition.Run(context.Background())
	assert.NoError(t, err)

	// Verify NO policy was created
	policies, err := client.NetworkingV1().NetworkPolicies("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, policies.Items, 0)
}

func TestParseLabelSelector(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]string
	}{
		{"", map[string]string{}},
		{"app=web", map[string]string{"app": "web"}},
		{"app=web,tier=frontend", map[string]string{"app": "web", "tier": "frontend"}},
		{"app = web , tier = frontend", map[string]string{"app": "web", "tier": "frontend"}},
	}

	for _, tt := range tests {
		got := parseLabelSelector(tt.input)
		assert.Equal(t, tt.want, got)
	}
}
