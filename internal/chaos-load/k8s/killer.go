// Package k8s provides Kubernetes chaos engineering operations.
package k8s

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/neogan/sre-toolkit/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KillerConfig configures pod killing behavior.
type KillerConfig struct {
	Namespace     string
	LabelSelector string
	GracePeriod   time.Duration // 0 means force kill (grace period = 0s)
	Interval      time.Duration // time between kills
	Count         int           // 0 = kill once, >0 = kill N times
	DryRun        bool
}

// PodKiller selects and terminates pods matching the given selector.
type PodKiller struct {
	client kubernetes.Interface
	config KillerConfig
}

// NewPodKiller creates a new PodKiller.
func NewPodKiller(client kubernetes.Interface, cfg KillerConfig) *PodKiller {
	return &PodKiller{client: client, config: cfg}
}

// Run executes the pod killing scenario.
func (k *PodKiller) Run(ctx context.Context) error {
	logger := logging.GetLogger()
	iterations := k.config.Count
	if iterations == 0 {
		iterations = 1
	}

	for i := 0; i < iterations; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(k.config.Interval):
			}
		}

		pod, err := k.pickRandomPod(ctx)
		if err != nil {
			return err
		}

		if k.config.DryRun {
			logger.Info().
				Str("pod", pod.Name).
				Str("namespace", pod.Namespace).
				Msg("[dry-run] would kill pod")
			continue
		}

		if err := k.killPod(ctx, pod); err != nil {
			return err
		}

		logger.Info().
			Str("pod", pod.Name).
			Str("namespace", pod.Namespace).
			Bool("force", k.config.GracePeriod == 0).
			Msg("Pod killed")
	}

	return nil
}

func (k *PodKiller) pickRandomPod(ctx context.Context) (*corev1.Pod, error) {
	opts := metav1.ListOptions{LabelSelector: k.config.LabelSelector}
	list, err := k.client.CoreV1().Pods(k.config.Namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	running := make([]corev1.Pod, 0, len(list.Items))
	for _, p := range list.Items {
		if p.Status.Phase == corev1.PodRunning {
			running = append(running, p)
		}
	}

	if len(running) == 0 {
		return nil, fmt.Errorf("no running pods found in namespace %q with selector %q",
			k.config.Namespace, k.config.LabelSelector)
	}

	return &running[rand.Intn(len(running))], nil
}

func (k *PodKiller) killPod(ctx context.Context, pod *corev1.Pod) error {
	gracePeriod := int64(k.config.GracePeriod.Seconds())
	opts := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}
	if err := k.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, opts); err != nil {
		return fmt.Errorf("failed to delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	return nil
}
