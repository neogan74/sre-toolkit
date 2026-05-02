package healthcheck

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckNetworkPolicies(t *testing.T) {
	// Setup fake client with namespaces and network policies
	clientset := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-system",
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-with-policies",
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-without-policies",
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-all",
				Namespace: "app-with-policies",
			},
		},
	)

	ctx := context.TODO()

	tests := []struct {
		name             string
		namespace        string
		expectedLen      int
		expectedPolicies int
		expectedNSCount  int
		expectedIssueNs  string
	}{
		{
			name:             "All namespaces",
			namespace:        "",
			expectedLen:      2, // default and app-without-policies (kube-system is excluded)
			expectedPolicies: 1,
			expectedNSCount:  3, // default, app-with-policies, app-without-policies
			expectedIssueNs:  "default",
		},
		{
			name:             "Specific namespace without policies",
			namespace:        "app-without-policies",
			expectedLen:      1,
			expectedPolicies: 0,
			expectedNSCount:  1,
			expectedIssueNs:  "app-without-policies",
		},
		{
			name:             "Specific namespace with policies",
			namespace:        "app-with-policies",
			expectedLen:      0,
			expectedPolicies: 1,
			expectedNSCount:  1,
			expectedIssueNs:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := CheckNetworkPolicies(ctx, clientset, tt.namespace)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedNSCount, status.TotalNamespaces, "TotalNamespaces mismatch")
			assert.Equal(t, tt.expectedPolicies, status.TotalPolicies, "TotalPolicies mismatch")
			assert.Equal(t, tt.expectedLen, len(status.Issues), "Issues length mismatch")

			if tt.expectedIssueNs != "" && len(status.Issues) > 0 {
				found := false
				for _, issue := range status.Issues {
					if issue.Namespace == tt.expectedIssueNs {
						found = true
						assert.Equal(t, "Warning", issue.Severity)
						assert.Contains(t, issue.Message, "has no NetworkPolicies defined")
						break
					}
				}
				assert.True(t, found, "Expected issue for namespace %s not found", tt.expectedIssueNs)
			}
		})
	}
}

func TestIsSystemNamespace(t *testing.T) {
	tests := []struct {
		namespace string
		expected  bool
	}{
		{"kube-system", true},
		{"kube-public", true},
		{"kube-node-lease", true},
		{"local-path-storage", true},
		{"cert-manager", true},
		{"ingress-nginx", true},
		{"kube-custom", true}, // Prefix match
		{"default", false},
		{"app-namespace", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSystemNamespace(tt.namespace))
		})
	}
}
