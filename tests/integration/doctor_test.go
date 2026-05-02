package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestDiagnosticsWithFailure(t *testing.T) {
	// Create a failing pod
	t.Log("Creating a failing pod...")
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "run", "failing-pod", "--image=busybox", "--", "sh", "-c", "exit 1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create failing pod: %v\nOutput: %s", err, output)
	}

	// Defer cleanup
	defer func() {
		_ = exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "pod", "failing-pod", "--force", "--grace-period=0").Run()
	}()

	// Wait for pod to enter Error/CrashLoopBackOff or just wait a few seconds
	t.Log("Waiting for pod to manifest error...")
	time.Sleep(10 * time.Second)

	rootDir, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("Failed to get root dir: %v", err)
	}

	cmd = exec.Command("go", "run", "./cmd/k8s-doctor", "diagnostics", "--kubeconfig", kubeconfigPath)
	cmd.Dir = rootDir
	output, err = cmd.CombinedOutput()
	// We expect err != nil if there are critical issues, as k8s-doctor exits with 1.
	// We only fail the test if there was a problem running the command itself.
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("Diagnostics command failed to execute: %v\nOutput: %s", err, output)
		}
	}

	t.Logf("Diagnostics output:\n%s\n", output)

	// Assertions
	outputStr := string(output)
	if !contains(outputStr, "failing-pod") {
		t.Error("Expected failing-pod to be mentioned in diagnostics report")
	}
	if !contains(outputStr, "🔴") && !contains(outputStr, "⚠️") {
		t.Error("Expected error/warning emoji in output")
	}
}

func TestAudit(t *testing.T) {
	rootDir, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("Failed to get root dir: %v", err)
	}

	// Case 1: Healthy cluster audit
	doctorPath := filepath.Join(rootDir, "bin", "k8s-doctor")
	cmd := exec.Command(doctorPath, "audit", "--kubeconfig", kubeconfigPath)
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	// Audit might return 1 if there are warnings (like missing network policies in default)
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("Audit command failed to execute: %v\nOutput: %s", err, output)
		}
	}

	t.Logf("Initial audit output:\n%s\n", output)

	// Case 2: Create a dangerously over-privileged role
	t.Log("Creating over-privileged cluster role...")
	manifest := `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dangerous-role
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: dangerous-binding
subjects:
- kind: ServiceAccount
  name: default
  namespace: default
roleRef:
  kind: ClusterRole
  name: dangerous-role
  apiGroup: rbac.authorization.k8s.io
`
	tmpfile, err := os.CreateTemp("", "manifest-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	applyCmd := exec.Command("kubectl", "apply", "-f", tmpfile.Name(), "--kubeconfig", kubeconfigPath)
	if out, err := applyCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to apply dangerous role: %v\nOutput: %s", err, out)
	}

	// Wait a bit for it to settle
	time.Sleep(2 * time.Second)

	// Run audit again
	cmd = exec.Command(doctorPath, "audit", "--kubeconfig", kubeconfigPath)
	cmd.Dir = rootDir
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected audit to fail with critical issues, but it succeeded")
	}

	t.Logf("Audit with dangerous role output:\n%s\n", output)

	// Verify the output contains the expected critical issues
	if !strings.Contains(string(output), "dangerous-role") {
		t.Errorf("Expected output to contain 'dangerous-role', but got:\n%s", output)
	}
	if !strings.Contains(string(output), "dangerous execution verbs") {
		t.Errorf("Expected output to contain 'dangerous execution verbs', but got:\n%s", output)
	}

	// Case 3: Missing resource quota in a new namespace
	t.Log("Checking missing resource quota...")
	// Delete first to ensure a clean state
	exec.Command("kubectl", "delete", "namespace", "empty-ns", "--kubeconfig", kubeconfigPath, "--ignore-not-found").Run()

	nsCmd := exec.Command("kubectl", "create", "namespace", "empty-ns", "--kubeconfig", kubeconfigPath)
	if out, err := nsCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create namespace: %v\nOutput: %s", err, out)
	}

	// Run audit ONLY for empty-ns with JSON output
	cmd = exec.Command(doctorPath, "audit", "-n", "empty-ns", "-o", "json", "--kubeconfig", kubeconfigPath)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Expected audit to fail in empty-ns due to missing quota, but it succeeded. Output:\n%s", output)
	}

	// Verify JSON structure and that empty-ns is caught
	var res struct {
		ResourceQuotaIssues []struct {
			Namespace string
			Severity  string
			Message   string
		}
	}

	// Output contains log lines before and after JSON
	start := strings.Index(string(output), "{")
	end := strings.LastIndex(string(output), "}")
	if start == -1 || end == -1 || end < start {
		t.Fatalf("Failed to find JSON in output:\n%s", output)
	}

	if err := json.Unmarshal(output[start:end+1], &res); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v\nOutput:\n%s", err, output)
	}

	found := false
	for _, issue := range res.ResourceQuotaIssues {
		if issue.Namespace == "empty-ns" && strings.Contains(issue.Message, "ResourceQuota") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find ResourceQuota issue for empty-ns in JSON, but got: %+v", res)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
