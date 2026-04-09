package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/neogan/sre-toolkit/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DrainerConfig configures node draining behavior.
type DrainerConfig struct {
	NodeName           string
	GracePeriod        time.Duration
	Timeout            time.Duration
	IgnoreDaemonSets   bool
	DeleteEmptyDirData bool
	DryRun             bool
}

// NodeDrainer cordons and evicts all pods from a node.
type NodeDrainer struct {
	client kubernetes.Interface
	config DrainerConfig
}

// NewNodeDrainer creates a new NodeDrainer.
func NewNodeDrainer(client kubernetes.Interface, cfg DrainerConfig) *NodeDrainer {
	return &NodeDrainer{client: client, config: cfg}
}

// Run cordons the node then evicts all evictable pods.
func (d *NodeDrainer) Run(ctx context.Context) error {
	logger := logging.GetLogger()

	if d.config.DryRun {
		logger.Info().Str("node", d.config.NodeName).Msg("[dry-run] would cordon and drain node")
		return nil
	}

	// Cordon the node
	if err := d.cordon(ctx); err != nil {
		return err
	}
	logger.Info().Str("node", d.config.NodeName).Msg("Node cordoned")

	// List pods on the node
	pods, err := d.listEvictablePods(ctx)
	if err != nil {
		return err
	}

	logger.Info().Str("node", d.config.NodeName).Int("pods", len(pods)).Msg("Evicting pods")

	drainCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	for _, pod := range pods {
		if err := d.evict(drainCtx, pod); err != nil {
			return fmt.Errorf("failed to evict pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}
		logger.Info().
			Str("pod", pod.Name).
			Str("namespace", pod.Namespace).
			Msg("Pod evicted")
	}

	return nil
}

func (d *NodeDrainer) cordon(ctx context.Context) error {
	node, err := d.client.CoreV1().Nodes().Get(ctx, d.config.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", d.config.NodeName, err)
	}

	if node.Spec.Unschedulable {
		return nil // already cordoned
	}

	node.Spec.Unschedulable = true
	if _, err := d.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to cordon node %s: %w", d.config.NodeName, err)
	}
	return nil
}

func (d *NodeDrainer) listEvictablePods(ctx context.Context) ([]corev1.Pod, error) {
	allPods, err := d.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + d.config.NodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods on node %s: %w", d.config.NodeName, err)
	}

	evictable := make([]corev1.Pod, 0, len(allPods.Items))
	for _, pod := range allPods.Items {
		if d.shouldSkip(pod) {
			continue
		}
		evictable = append(evictable, pod)
	}
	return evictable, nil
}

func (d *NodeDrainer) shouldSkip(pod corev1.Pod) bool {
	// Skip mirror pods (static pods managed by kubelet)
	if _, ok := pod.Annotations[corev1.MirrorPodAnnotationKey]; ok {
		return true
	}

	// Skip DaemonSet pods if configured
	if d.config.IgnoreDaemonSets {
		for _, ref := range pod.OwnerReferences {
			if ref.Kind == "DaemonSet" {
				return true
			}
		}
	}

	// Skip pods with emptyDir if not configured to delete them
	if !d.config.DeleteEmptyDirData {
		for _, vol := range pod.Spec.Volumes {
			if vol.EmptyDir != nil {
				return true
			}
		}
	}

	return false
}

func (d *NodeDrainer) evict(ctx context.Context, pod corev1.Pod) error {
	gracePeriod := int64(d.config.GracePeriod.Seconds())
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		},
	}

	err := d.client.PolicyV1().Evictions(pod.Namespace).Evict(ctx, eviction)
	if errors.IsNotFound(err) {
		return nil // pod already gone
	}
	return err
}
