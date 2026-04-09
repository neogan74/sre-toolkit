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

func testNode(name string, unschedulable bool) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Unschedulable: unschedulable},
	}
}

func podOnNode(namespace, name, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       corev1.PodSpec{NodeName: nodeName},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestNodeDrainer_DryRun(t *testing.T) {
	client := fake.NewSimpleClientset(
		testNode("node-1", false),
		podOnNode("default", "pod-a", "node-1"),
	)

	drainer := NewNodeDrainer(client, DrainerConfig{
		NodeName: "node-1",
		Timeout:  30 * time.Second,
		DryRun:   true,
	})

	require.NoError(t, drainer.Run(context.Background()))

	// Node must still be schedulable
	node, err := client.CoreV1().Nodes().Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, node.Spec.Unschedulable, "dry-run should not cordon node")
}

func TestNodeDrainer_CordonsNode(t *testing.T) {
	client := fake.NewSimpleClientset(testNode("node-1", false))

	drainer := NewNodeDrainer(client, DrainerConfig{
		NodeName:           "node-1",
		Timeout:            10 * time.Second,
		IgnoreDaemonSets:   true,
		DeleteEmptyDirData: true,
	})

	require.NoError(t, drainer.Run(context.Background()))

	node, err := client.CoreV1().Nodes().Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, node.Spec.Unschedulable, "node should be cordoned")
}

func TestNodeDrainer_AlreadyCordoned(t *testing.T) {
	client := fake.NewSimpleClientset(testNode("node-1", true))

	drainer := NewNodeDrainer(client, DrainerConfig{
		NodeName:           "node-1",
		Timeout:            10 * time.Second,
		IgnoreDaemonSets:   true,
		DeleteEmptyDirData: true,
	})

	require.NoError(t, drainer.Run(context.Background()))
}

func TestNodeDrainer_SkipsDaemonSetPods(t *testing.T) {
	ds := podOnNode("kube-system", "ds-pod", "node-1")
	ds.OwnerReferences = []metav1.OwnerReference{{Kind: "DaemonSet", Name: "fluentd"}}

	client := fake.NewSimpleClientset(
		testNode("node-1", false),
		ds,
	)

	drainer := NewNodeDrainer(client, DrainerConfig{
		NodeName:           "node-1",
		Timeout:            10 * time.Second,
		IgnoreDaemonSets:   true,
		DeleteEmptyDirData: true,
	})

	require.NoError(t, drainer.Run(context.Background()))
}

func TestNodeDrainer_SkipsMirrorPods(t *testing.T) {
	mirror := podOnNode("kube-system", "etcd-node-1", "node-1")
	mirror.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "true"}

	client := fake.NewSimpleClientset(
		testNode("node-1", false),
		mirror,
	)

	drainer := NewNodeDrainer(client, DrainerConfig{
		NodeName:           "node-1",
		Timeout:            10 * time.Second,
		IgnoreDaemonSets:   true,
		DeleteEmptyDirData: true,
	})

	require.NoError(t, drainer.Run(context.Background()))
}

func TestNodeDrainer_SkipsEmptyDirPodsWhenNotConfigured(t *testing.T) {
	pod := podOnNode("default", "stateful-pod", "node-1")
	pod.Spec.Volumes = []corev1.Volume{
		{Name: "cache", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}

	client := fake.NewSimpleClientset(
		testNode("node-1", false),
		pod,
	)

	drainer := NewNodeDrainer(client, DrainerConfig{
		NodeName:           "node-1",
		Timeout:            10 * time.Second,
		DeleteEmptyDirData: false, // should skip emptyDir pods
	})

	require.NoError(t, drainer.Run(context.Background()))
}

func TestNodeDrainer_NodeNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	drainer := NewNodeDrainer(client, DrainerConfig{
		NodeName: "nonexistent",
		Timeout:  5 * time.Second,
	})

	err := drainer.Run(context.Background())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to get node")
}
