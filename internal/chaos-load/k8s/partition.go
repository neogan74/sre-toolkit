package k8s

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/neogan/sre-toolkit/pkg/logging"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NetworkPartitionConfig configures the network partition behavior.
type NetworkPartitionConfig struct {
	Namespace     string
	LabelSelector string
	Duration      time.Duration
	DryRun        bool
}

// NetworkPartition isolates pods matching the given selector.
type NetworkPartition struct {
	client kubernetes.Interface
	config NetworkPartitionConfig
}

// NewNetworkPartition creates a new NetworkPartition.
func NewNetworkPartition(client kubernetes.Interface, cfg NetworkPartitionConfig) *NetworkPartition {
	return &NetworkPartition{client: client, config: cfg}
}

// Run executes the network partition scenario.
func (p *NetworkPartition) Run(ctx context.Context) error {
	logger := logging.GetLogger()

	policyName := fmt.Sprintf("chaos-network-partition-%d", time.Now().UnixNano())

	if p.config.DryRun {
		logger.Info().
			Str("namespace", p.config.Namespace).
			Str("selector", p.config.LabelSelector).
			Dur("duration", p.config.Duration).
			Msg("[dry-run] would apply network partition policy")
		return nil
	}

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: p.config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "chaos-load",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: parseLabelSelector(p.config.LabelSelector),
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	}

	logger.Info().
		Str("namespace", p.config.Namespace).
		Str("selector", p.config.LabelSelector).
		Str("policy", policyName).
		Msg("Creating network partition policy (Default Deny All)")

	_, err := p.client.NetworkingV1().NetworkPolicies(p.config.Namespace).Create(ctx, policy, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create network policy: %w", err)
	}

	// Ensure cleanup on interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// We use a separate context for cleanup to ensure it runs even if the parent ctx is canceled
	cleanup := func() {
		logger.Info().Str("policy", policyName).Msg("Cleaning up network partition policy")
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.client.NetworkingV1().NetworkPolicies(p.config.Namespace).Delete(cleanupCtx, policyName, metav1.DeleteOptions{}); err != nil {
			logger.Error().Err(err).Msg("Failed to clean up network policy")
		}
	}

	defer func() {
		signal.Stop(sigChan)
		cleanup()
	}()

	logger.Info().Dur("duration", p.config.Duration).Msg("Waiting for partition duration to end")

	select {
	case <-ctx.Done():
		logger.Info().Msg("Context canceled, ending partition early")
	case <-sigChan:
		logger.Info().Msg("Interrupt received, ending partition early")
	case <-time.After(p.config.Duration):
		logger.Info().Msg("Partition duration elapsed")
	}

	return nil
}

// parseLabelSelector is a simple helper to parse 'key=value' comma separated selectors
func parseLabelSelector(selector string) map[string]string {
	labels := make(map[string]string)
	if selector == "" {
		return labels
	}

	pairs := splitTrim(selector, ",")
	for _, pair := range pairs {
		kv := splitTrim(pair, "=")
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}
	return labels
}

func splitTrim(s string, sep string) []string {
	var res []string
	parts := strings.Split(s, sep)
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
