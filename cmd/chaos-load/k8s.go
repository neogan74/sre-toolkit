package main

import (
	"time"

	k8schaos "github.com/neogan/sre-toolkit/internal/chaos-load/k8s"
	"github.com/neogan/sre-toolkit/pkg/k8s"
	"github.com/spf13/cobra"
)

func newK8sCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k8s",
		Short: "Kubernetes chaos scenarios",
		Long:  "Simulate failures in a Kubernetes cluster (pod kills, node drains).",
	}

	cmd.AddCommand(newPodKillCmd())
	cmd.AddCommand(newNodeDrainCmd())

	return cmd
}

func newPodKillCmd() *cobra.Command {
	var (
		kubeconfig    string
		namespace     string
		labelSelector string
		gracePeriod   time.Duration
		interval      time.Duration
		count         int
		dryRun        bool
	)

	cmd := &cobra.Command{
		Use:   "pod-kill",
		Short: "Kill random pods matching a label selector",
		Long: `Randomly terminates running pods that match the given label selector.
Useful for testing how your workloads recover from unexpected pod failures.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := k8s.NewClient(&k8s.Config{Kubeconfig: kubeconfig})
			if err != nil {
				return err
			}

			killer := k8schaos.NewPodKiller(client.Clientset(), k8schaos.KillerConfig{
				Namespace:     namespace,
				LabelSelector: labelSelector,
				GracePeriod:   gracePeriod,
				Interval:      interval,
				Count:         count,
				DryRun:        dryRun,
			})

			return killer.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to target")
	cmd.Flags().StringVarP(&labelSelector, "selector", "l", "", "Label selector (e.g. app=web)")
	cmd.Flags().DurationVar(&gracePeriod, "grace-period", 30*time.Second, "Termination grace period (0 for force kill)")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Time between kills when --count > 1")
	cmd.Flags().IntVar(&count, "count", 1, "Number of pods to kill (sequentially)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be killed without actually killing")

	return cmd
}

func newNodeDrainCmd() *cobra.Command {
	var (
		kubeconfig         string
		nodeName           string
		gracePeriod        time.Duration
		timeout            time.Duration
		ignoreDaemonSets   bool
		deleteEmptyDirData bool
		dryRun             bool
	)

	cmd := &cobra.Command{
		Use:   "node-drain",
		Short: "Cordon and drain a Kubernetes node",
		Long: `Marks a node as unschedulable (cordon) then evicts all eligible pods.
Mirrors the behavior of kubectl drain with configurable safety options.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if nodeName == "" {
				return cmd.Usage()
			}

			client, err := k8s.NewClient(&k8s.Config{Kubeconfig: kubeconfig})
			if err != nil {
				return err
			}

			drainer := k8schaos.NewNodeDrainer(client.Clientset(), k8schaos.DrainerConfig{
				NodeName:           nodeName,
				GracePeriod:        gracePeriod,
				Timeout:            timeout,
				IgnoreDaemonSets:   ignoreDaemonSets,
				DeleteEmptyDirData: deleteEmptyDirData,
				DryRun:             dryRun,
			})

			return drainer.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVar(&nodeName, "node", "", "Name of the node to drain")
	cmd.Flags().DurationVar(&gracePeriod, "grace-period", 30*time.Second, "Pod termination grace period")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Timeout for the drain operation")
	cmd.Flags().BoolVar(&ignoreDaemonSets, "ignore-daemonsets", true, "Ignore DaemonSet-managed pods")
	cmd.Flags().BoolVar(&deleteEmptyDirData, "delete-emptydir-data", false, "Delete local data in emptyDir volumes")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be drained without taking action")

	cmd.MarkFlagRequired("node")

	return cmd
}
