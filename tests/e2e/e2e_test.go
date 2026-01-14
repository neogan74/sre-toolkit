package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	clusterName = "k8s-doctor-e2e"
	connRetry   = 10
	connWait    = 5 * time.Second
)

var (
	kubeconfigPath string
)

func TestMain(m *testing.M) {
	// Setup
	var err error
	kubeconfigPath, err = filepath.Abs("kubeconfig")
	if err != nil {
		fmt.Printf("Failed to get absolute path for kubeconfig: %v\n", err)
		os.Exit(1)
	}

	if err := createCluster(); err != nil {
		fmt.Printf("Failed to create cluster: %v\n", err)
		// Try to clean up partially created cluster
		_ = deleteCluster()
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown
	if err := deleteCluster(); err != nil {
		fmt.Printf("Failed to delete cluster: %v\n", err)
	}

	os.Exit(code)
}

func createCluster() error {
	fmt.Printf("Creating Kind cluster %s...\n", clusterName)
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--kubeconfig", kubeconfigPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create kind cluster: %v, output: %s", err, output)
	}

	// Wait for nodes to be ready
	fmt.Println("Waiting for nodes to be ready...")
	// We need to retry waiting because API server might not be immediately available after create returns
	for i := 0; i < connRetry; i++ {
		cmd = exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "wait", "--for=condition=Ready", "nodes", "--all", "--timeout=2m")
		output, err := cmd.CombinedOutput()
		if err == nil {
			fmt.Println("Nodes are ready!")
			break
		}
		if i == connRetry-1 {
			fmt.Printf("kubectl wait output: %s\n", output)
			return fmt.Errorf("timeout waiting for nodes to be ready: %v", err)
		}
		time.Sleep(connWait)
	}

	// Wait for pods to be ready (kube-system)
	fmt.Println("Waiting for kube-system pods to be ready...")
	cmd = exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "wait", "--for=condition=Ready", "pods", "--all", "-n", "kube-system", "--timeout=2m")
	// We don't error here strictly because some pods might be tricky (like coredns needing network)
	// but mostly if nodes are ready, CNI is ready.
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: kube-system pods not fully ready: %s\n", output)
	} else {
		fmt.Println("kube-system pods are ready!")
	}

	fmt.Println("Cluster is ready!")
	return nil
}

func deleteCluster() error {
	fmt.Printf("Deleting Kind cluster %s...\n", clusterName)
	cmd := exec.Command("kind", "delete", "cluster", "--name", clusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete kind cluster: %v, output: %s", err, output)
	}

	// Remove kubeconfig file
	if err := os.Remove(kubeconfigPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove kubeconfig file: %v", err)
	}

	return nil
}

func TestHealthCheck(t *testing.T) {
	// Build the k8s-doctor binary first?
	// Or just run `go run`? Using `go run` is slower but easier for tests.
	// Let's assume we compile it or use go run. Let's use go run for simplicity in dev loop.

	// We need to run from the root of the repo to find go.mod if we use go run
	rootDir, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("Failed to get root dir: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/k8s-doctor", "healthcheck", "--kubeconfig", kubeconfigPath, "--timeout", "1m")
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Healthcheck command failed: %v\nOutput: %s", err, output)
	}

	fmt.Printf("Healthcheck output:\n%s\n", output)
	// Add assertions here based on expected output for a healthy kind cluster
}

func TestDiagnostics(t *testing.T) {
	rootDir, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("Failed to get root dir: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/k8s-doctor", "diagnostics", "--kubeconfig", kubeconfigPath)
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Diagnostics command failed: %v\nOutput: %s", err, output)
	}

	fmt.Printf("Diagnostics output:\n%s\n", output)
}
