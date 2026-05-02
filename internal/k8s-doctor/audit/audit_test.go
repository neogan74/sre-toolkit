package audit

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRunAudit(t *testing.T) {
	tests := []struct {
		name                string
		namespace           string
		objects             []runtime.Object
		wantResources       int
		wantProbes          int
		wantSecurity        int
		wantRBAC            int
		wantResourceQuotas  int
		wantNetworkPolicies int
		wantCritical        int
		wantWarning         int
	}{
		{
			name: "healthy namespace",
			objects: []runtime.Object{
				makeNamespace("default"),
				makeHealthyPod("app", "default"),
				makeResourceQuota("default", "compute-quota"),
			},
			wantResources:       0,
			wantProbes:          0,
			wantSecurity:        0,
			wantRBAC:            0,
			wantResourceQuotas:  0,
			wantNetworkPolicies: 1,
			wantCritical:        0,
			wantWarning:         1,
		},
		{
			name:      "namespace filter limits scope",
			namespace: "default",
			objects: []runtime.Object{
				makeNamespace("default"),
				makeNamespace("other"),
				makeBrokenPod("app", "default"),
				makeBrokenPod("app", "other"),
				makeResourceQuota("default", "compute-quota"),
			},
			wantResources:       4,
			wantProbes:          2,
			wantSecurity:        2,
			wantRBAC:            0,
			wantResourceQuotas:  0,
			wantNetworkPolicies: 1,
			wantCritical:        2,
			wantWarning:         7,
		},
		{
			name: "network policy suppresses warning",
			objects: []runtime.Object{
				makeNamespace("secure"),
				makeHealthyPod("app", "secure"),
				makeNetworkPolicy("default-deny", "secure"),
				makeResourceQuota("secure", "compute-quota"),
			},
			wantResources:       0,
			wantProbes:          0,
			wantSecurity:        0,
			wantRBAC:            0,
			wantResourceQuotas:  0,
			wantNetworkPolicies: 0,
			wantCritical:        0,
			wantWarning:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.objects...)

			got, err := RunAudit(context.Background(), clientset, tt.namespace)
			if err != nil {
				t.Fatalf("RunAudit() error = %v", err)
			}

			if len(got.ResourceIssues) != tt.wantResources {
				t.Fatalf("RunAudit() resource issues = %d, want %d", len(got.ResourceIssues), tt.wantResources)
			}
			if len(got.ProbeIssues) != tt.wantProbes {
				t.Fatalf("RunAudit() probe issues = %d, want %d", len(got.ProbeIssues), tt.wantProbes)
			}
			if len(got.SecurityIssues) != tt.wantSecurity {
				t.Fatalf("RunAudit() security issues = %d, want %d", len(got.SecurityIssues), tt.wantSecurity)
			}
			if len(got.RBACIssues) != tt.wantRBAC {
				t.Fatalf("RunAudit() RBAC issues = %d, want %d", len(got.RBACIssues), tt.wantRBAC)
			}
			if len(got.ResourceQuotaIssues) != tt.wantResourceQuotas {
				t.Fatalf("RunAudit() resource quota issues = %d, want %d", len(got.ResourceQuotaIssues), tt.wantResourceQuotas)
			}
			if len(got.NetworkPolicyIssues) != tt.wantNetworkPolicies {
				t.Fatalf("RunAudit() network policy issues = %d, want %d", len(got.NetworkPolicyIssues), tt.wantNetworkPolicies)
			}
			if got.Summary.CriticalCount != tt.wantCritical {
				t.Fatalf("RunAudit() critical count = %d, want %d", got.Summary.CriticalCount, tt.wantCritical)
			}
			if got.Summary.WarningCount != tt.wantWarning {
				t.Fatalf("RunAudit() warning count = %d, want %d", got.Summary.WarningCount, tt.wantWarning)
			}
		})
	}
}

func TestRunAuditRBAC(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		objects      []runtime.Object
		wantRBAC     int
		wantCritical int
		wantWarning  int
	}{
		{
			name: "cluster wide wildcard role and binding are critical",
			objects: []runtime.Object{
				makeClusterRole("platform-admin", []rbacv1.PolicyRule{
					{Resources: []string{"*"}, Verbs: []string{"*"}},
				}),
				makeClusterRoleBinding("platform-admin-binding", "platform-admin", rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "deployer",
					Namespace: "default",
				}),
			},
			wantRBAC:     2,
			wantCritical: 2,
			wantWarning:  0,
		},
		{
			name:      "namespace scoped audit ignores out of scope bindings",
			namespace: "default",
			objects: []runtime.Object{
				makeNetworkPolicy("default-deny", "default"),
				makeResourceQuota("default", "compute-quota"),
				makeRole("default", "secret-reader", []rbacv1.PolicyRule{
					{Resources: []string{"secrets"}, Verbs: []string{"get", "list"}},
				}),
				makeRoleBinding("default", "secret-reader-binding", "Role", "secret-reader", rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "app",
					Namespace: "default",
				}),
				makeRole("other", "secret-reader", []rbacv1.PolicyRule{
					{Resources: []string{"secrets"}, Verbs: []string{"get", "list"}},
				}),
				makeRoleBinding("other", "secret-reader-binding", "Role", "secret-reader", rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "app",
					Namespace: "other",
				}),
			},
			wantRBAC:     2,
			wantCritical: 0,
			wantWarning:  2,
		},
		{
			name:      "cluster admin binding for namespace service account is detected",
			namespace: "team-a",
			objects: []runtime.Object{
				makeNetworkPolicy("default-deny", "team-a"),
				makeResourceQuota("team-a", "compute-quota"),
				makeClusterRoleBinding("team-a-admin", "cluster-admin", rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "runner",
					Namespace: "team-a",
				}),
				makeClusterRoleBinding("team-b-admin", "cluster-admin", rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "runner",
					Namespace: "team-b",
				}),
			},
			wantRBAC:     1,
			wantCritical: 1,
			wantWarning:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.objects...)

			got, err := RunAudit(context.Background(), clientset, tt.namespace)
			if err != nil {
				t.Fatalf("RunAudit() error = %v", err)
			}

			if len(got.RBACIssues) != tt.wantRBAC {
				t.Fatalf("RunAudit() RBAC issues = %d, want %d", len(got.RBACIssues), tt.wantRBAC)
			}
			if got.Summary.CriticalCount != tt.wantCritical {
				t.Fatalf("RunAudit() critical count = %d, want %d", got.Summary.CriticalCount, tt.wantCritical)
			}
			if got.Summary.WarningCount != tt.wantWarning {
				t.Fatalf("RunAudit() warning count = %d, want %d", got.Summary.WarningCount, tt.wantWarning)
			}
		})
	}
}

func TestRunAuditResourceQuotas(t *testing.T) {
	tests := []struct {
		name               string
		namespace          string
		objects            []runtime.Object
		wantResourceQuotas int
		wantWarning        int
	}{
		{
			name: "missing quota in target namespace produces warning",
			objects: []runtime.Object{
				makeNamespace("default"),
				makeNetworkPolicy("default-deny", "default"),
			},
			wantResourceQuotas: 1,
			wantWarning:        1,
		},
		{
			name:      "namespace filter ignores quotas outside scope",
			namespace: "team-a",
			objects: []runtime.Object{
				makeNamespace("team-a"),
				makeNamespace("team-b"),
				makeNetworkPolicy("default-deny", "team-a"),
				makeResourceQuota("team-b", "compute-quota"),
			},
			wantResourceQuotas: 1,
			wantWarning:        1,
		},
		{
			name: "system namespaces are skipped in cluster wide audit",
			objects: []runtime.Object{
				makeNamespace("default"),
				makeNamespace("kube-system"),
				makeNetworkPolicy("default-deny", "default"),
				makeResourceQuota("default", "compute-quota"),
			},
			wantResourceQuotas: 0,
			wantWarning:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.objects...)

			got, err := RunAudit(context.Background(), clientset, tt.namespace)
			if err != nil {
				t.Fatalf("RunAudit() error = %v", err)
			}

			if len(got.ResourceQuotaIssues) != tt.wantResourceQuotas {
				t.Fatalf("RunAudit() resource quota issues = %d, want %d", len(got.ResourceQuotaIssues), tt.wantResourceQuotas)
			}
			if got.Summary.WarningCount != tt.wantWarning {
				t.Fatalf("RunAudit() warning count = %d, want %d", got.Summary.WarningCount, tt.wantWarning)
			}
		})
	}
}

func makeNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func makeNetworkPolicy(name, namespace string) runtime.Object {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func makeResourceQuota(namespace, name string) runtime.Object {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func makeHealthyPod(name, namespace string) *corev1.Pod {
	runAsNonRoot := true
	readOnlyRootFilesystem := true

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					LivenessProbe:  &corev1.Probe{},
					ReadinessProbe: &corev1.Probe{},
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
					},
				},
			},
		},
	}
}

func makeBrokenPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
				},
			},
		},
	}
}

func makeRole(namespace, name string, rules []rbacv1.PolicyRule) runtime.Object {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: rules,
	}
}

func makeClusterRole(name string, rules []rbacv1.PolicyRule) runtime.Object {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}
}

func makeRoleBinding(namespace, name, roleKind, roleName string, subjects ...rbacv1.Subject) runtime.Object {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     roleKind,
			Name:     roleName,
		},
		Subjects: subjects,
	}
}

func makeClusterRoleBinding(name, roleName string, subjects ...rbacv1.Subject) runtime.Object {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: subjects,
	}
}

// Tests for helper functions

func TestHasWildcard(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected bool
	}{
		{
			name:     "contains wildcard",
			values:   []string{"pods", "*", "services"},
			expected: true,
		},
		{
			name:     "no wildcard",
			values:   []string{"pods", "services"},
			expected: false,
		},
		{
			name:     "empty list",
			values:   []string{},
			expected: false,
		},
		{
			name:     "only wildcard",
			values:   []string{"*"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasWildcard(tt.values); got != tt.expected {
				t.Errorf("hasWildcard(%v) = %v, want %v", tt.values, got, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
		result   bool
	}{
		{
			name:     "value exists",
			values:   []string{"get", "list", "watch"},
			expected: "list",
			result:   true,
		},
		{
			name:     "value not exists",
			values:   []string{"get", "list", "watch"},
			expected: "delete",
			result:   false,
		},
		{
			name:     "empty list",
			values:   []string{},
			expected: "get",
			result:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.values, tt.expected); got != tt.result {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.values, tt.expected, got, tt.result)
			}
		})
	}
}

func TestEvaluateSecretAccess(t *testing.T) {
	tests := []struct {
		name          string
		rule          rbacv1.PolicyRule
		expectedSev   string
		expectedFound bool
	}{
		{
			name: "secret create access",
			rule: rbacv1.PolicyRule{
				Resources: []string{"secrets"},
				Verbs:     []string{"create"},
			},
			expectedSev:   "Critical",
			expectedFound: true,
		},
		{
			name: "secret get access",
			rule: rbacv1.PolicyRule{
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
			expectedSev:   "Warning",
			expectedFound: true,
		},
		{
			name: "secret list access",
			rule: rbacv1.PolicyRule{
				Resources: []string{"secrets"},
				Verbs:     []string{"list"},
			},
			expectedSev:   "Warning",
			expectedFound: true,
		},
		{
			name: "no secret access",
			rule: rbacv1.PolicyRule{
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
			expectedSev:   "",
			expectedFound: false,
		},
		{
			name: "wildcard resources with get",
			rule: rbacv1.PolicyRule{
				Resources: []string{"*"},
				Verbs:     []string{"get"},
			},
			expectedSev:   "Warning",
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sev, found := evaluateSecretAccess(tt.rule)
			if found != tt.expectedFound {
				t.Errorf("evaluateSecretAccess found = %v, want %v", found, tt.expectedFound)
			}
			if found && sev != tt.expectedSev {
				t.Errorf("evaluateSecretAccess severity = %v, want %v", sev, tt.expectedSev)
			}
		})
	}
}

func TestHasPVDestruction(t *testing.T) {
	tests := []struct {
		name     string
		rule     rbacv1.PolicyRule
		expected bool
	}{
		{
			name: "delete persistent volumes",
			rule: rbacv1.PolicyRule{
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"delete"},
			},
			expected: true,
		},
		{
			name: "patch persistent volume claims",
			rule: rbacv1.PolicyRule{
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"patch"},
			},
			expected: true,
		},
		{
			name: "wildcard with delete",
			rule: rbacv1.PolicyRule{
				Resources: []string{"*"},
				Verbs:     []string{"delete"},
			},
			expected: true,
		},
		{
			name: "no destructive verbs",
			rule: rbacv1.PolicyRule{
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list"},
			},
			expected: false,
		},
		{
			name: "wrong resource",
			rule: rbacv1.PolicyRule{
				Resources: []string{"pods"},
				Verbs:     []string{"delete"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasPVDestruction(tt.rule); got != tt.expected {
				t.Errorf("hasPVDestruction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatSubject(t *testing.T) {
	tests := []struct {
		name           string
		subject        rbacv1.Subject
		expectedFormat string
	}{
		{
			name: "ServiceAccount with namespace",
			subject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: "kube-system",
			},
			expectedFormat: "ServiceAccount/kube-system/default",
		},
		{
			name: "User without namespace",
			subject: rbacv1.Subject{
				Kind: "User",
				Name: "admin@example.com",
			},
			expectedFormat: "User/admin@example.com",
		},
		{
			name: "Group without namespace",
			subject: rbacv1.Subject{
				Kind: "Group",
				Name: "developers",
			},
			expectedFormat: "Group/developers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSubject(tt.subject); got != tt.expectedFormat {
				t.Errorf("formatSubject() = %q, want %q", got, tt.expectedFormat)
			}
		})
	}
}
