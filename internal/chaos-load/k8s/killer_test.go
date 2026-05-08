package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func runningPod(namespace, name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestPodKiller_KillsOneRandomPod(t *testing.T) {
	client := fake.NewSimpleClientset(
		runningPod("default", "pod-a", map[string]string{"app": "web"}),
		runningPod("default", "pod-b", map[string]string{"app": "web"}),
	)

	killer := NewPodKiller(client, KillerConfig{
		Namespace:     "default",
		LabelSelector: "app=web",
		GracePeriod:   5 * time.Second,
		Count:         1,
	})

	err := killer.Run(context.Background())
	require.NoError(t, err)

	remaining, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, remaining.Items, 1, "one pod should have been deleted")
}

func TestPodKiller_ForceKill(t *testing.T) {
	client := fake.NewSimpleClientset(
		runningPod("default", "pod-a", nil),
	)

	killer := NewPodKiller(client, KillerConfig{
		Namespace:   "default",
		GracePeriod: 0, // force kill
		Count:       1,
	})

	require.NoError(t, killer.Run(context.Background()))

	remaining, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, remaining.Items)
}

func TestPodKiller_DryRun(t *testing.T) {
	client := fake.NewSimpleClientset(
		runningPod("default", "pod-a", nil),
	)

	killer := NewPodKiller(client, KillerConfig{
		Namespace: "default",
		Count:     1,
		DryRun:    true,
	})

	require.NoError(t, killer.Run(context.Background()))

	// Pod must still exist
	remaining, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, remaining.Items, 1)
}

func TestPodKiller_NoPodsReturnsError(t *testing.T) {
	client := fake.NewSimpleClientset()

	killer := NewPodKiller(client, KillerConfig{
		Namespace: "default",
		Count:     1,
	})

	err := killer.Run(context.Background())
	assert.ErrorContains(t, err, "no running pods found")
}

func TestPodKiller_SkipsNonRunningPods(t *testing.T) {
	pending := runningPod("default", "pending-pod", nil)
	pending.Status.Phase = corev1.PodPending

	client := fake.NewSimpleClientset(pending)

	killer := NewPodKiller(client, KillerConfig{
		Namespace: "default",
		Count:     1,
	})

	err := killer.Run(context.Background())
	assert.ErrorContains(t, err, "no running pods found")
}

func TestPodKiller_MultipleIterations(t *testing.T) {
	client := fake.NewSimpleClientset(
		runningPod("default", "pod-1", nil),
		runningPod("default", "pod-2", nil),
		runningPod("default", "pod-3", nil),
	)

	killer := NewPodKiller(client, KillerConfig{
		Namespace: "default",
		Count:     2,
		Interval:  0,
	})

	require.NoError(t, killer.Run(context.Background()))

	remaining, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, remaining.Items, 1, "two out of three pods should be deleted")
}
