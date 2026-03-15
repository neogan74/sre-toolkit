// Package audit provides security and best-practices audit functionality.
package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Result represents the result of an audit run.
type Result struct {
	Summary             Summary
	ResourceIssues      []ResourceIssue
	ProbeIssues         []ProbeIssue
	SecurityIssues      []SecurityIssue
	RBACIssues          []RBACIssue
	ResourceQuotaIssues []ResourceQuotaIssue
	NetworkPolicyIssues []healthcheck.NetworkPolicyIssue
}

// Summary provides an overview of issues found.
type Summary struct {
	TotalIssues   int
	CriticalCount int
	WarningCount  int
	InfoCount     int
}

// ResourceIssue represents a resource configuration issue.
type ResourceIssue struct {
	Pod       string
	Namespace string
	Severity  string
	Message   string
}

// ProbeIssue represents a probe configuration issue.
type ProbeIssue struct {
	Pod       string
	Namespace string
	Severity  string
	Message   string
}

// SecurityIssue represents a security configuration issue.
type SecurityIssue struct {
	Pod       string
	Namespace string
	Severity  string
	Message   string
}

// RBACIssue represents an RBAC policy or binding issue.
type RBACIssue struct {
	Namespace string
	Resource  string
	Subject   string
	Severity  string
	Message   string
}

// ResourceQuotaIssue represents a namespace-level resource quota issue.
type ResourceQuotaIssue struct {
	Namespace string
	Severity  string
	Message   string
}

// RunAudit performs a namespace-scoped or cluster-wide audit.
func RunAudit(ctx context.Context, clientset kubernetes.Interface, namespace string) (*Result, error) {
	result := &Result{
		ResourceIssues:      []ResourceIssue{},
		ProbeIssues:         []ProbeIssue{},
		SecurityIssues:      []SecurityIssue{},
		RBACIssues:          []RBACIssue{},
		ResourceQuotaIssues: []ResourceQuotaIssue{},
		NetworkPolicyIssues: []healthcheck.NetworkPolicyIssue{},
	}

	pods, err := healthcheck.CheckPods(ctx, clientset, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check pods: %w", err)
	}

	for _, audit := range pods.ResourceAudit {
		for _, container := range audit.Containers {
			for _, issueMsg := range container.Issues {
				result.ResourceIssues = append(result.ResourceIssues, ResourceIssue{
					Pod:       audit.Pod,
					Namespace: audit.Namespace,
					Severity:  "Warning",
					Message:   fmt.Sprintf("Container %s: %s", container.Name, issueMsg),
				})
			}
		}
	}

	for _, audit := range pods.ProbeAudit {
		for _, container := range audit.Containers {
			for _, issueMsg := range container.Issues {
				result.ProbeIssues = append(result.ProbeIssues, ProbeIssue{
					Pod:       audit.Pod,
					Namespace: audit.Namespace,
					Severity:  "Warning",
					Message:   fmt.Sprintf("Container %s: %s", container.Name, issueMsg),
				})
			}
		}
	}

	for _, audit := range pods.SecurityAudit {
		for _, container := range audit.Containers {
			for _, issueMsg := range container.Issues {
				result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
					Pod:       audit.Pod,
					Namespace: audit.Namespace,
					Severity:  "Critical",
					Message:   fmt.Sprintf("Container %s: %s", container.Name, issueMsg),
				})
			}
		}
	}

	networkPolicies, err := healthcheck.CheckNetworkPolicies(ctx, clientset, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check network policies: %w", err)
	}
	result.NetworkPolicyIssues = networkPolicies.Issues

	rbacIssues, err := auditRBAC(ctx, clientset, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to audit RBAC: %w", err)
	}
	result.RBACIssues = rbacIssues

	resourceQuotaIssues, err := auditResourceQuotas(ctx, clientset, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to audit resource quotas: %w", err)
	}
	result.ResourceQuotaIssues = resourceQuotaIssues

	result.Summary = calculateSummary(result)

	return result, nil
}

func calculateSummary(result *Result) Summary {
	summary := Summary{}

	for _, issue := range result.ResourceIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.ProbeIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.SecurityIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.RBACIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.ResourceQuotaIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	for _, issue := range result.NetworkPolicyIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "Warning":
			summary.WarningCount++
		case "Info":
			summary.InfoCount++
		}
	}

	return summary
}

func auditResourceQuotas(ctx context.Context, clientset kubernetes.Interface, namespace string) ([]ResourceQuotaIssue, error) {
	namespaces, err := namespacesToAudit(ctx, clientset, namespace)
	if err != nil {
		return nil, err
	}

	issues := make([]ResourceQuotaIssue, 0, len(namespaces))
	for _, ns := range namespaces {
		quotas, err := clientset.CoreV1().ResourceQuotas(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list resource quotas in namespace %s: %w", ns, err)
		}

		if len(quotas.Items) == 0 {
			issues = append(issues, ResourceQuotaIssue{
				Namespace: ns,
				Severity:  "Warning",
				Message:   fmt.Sprintf("Namespace %s has no ResourceQuota defined", ns),
			})
		}
	}

	return issues, nil
}

func auditRBAC(ctx context.Context, clientset kubernetes.Interface, namespace string) ([]RBACIssue, error) {
	issues := []RBACIssue{}

	if namespace == "" {
		roles, err := clientset.RbacV1().Roles(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list roles: %w", err)
		}
		for _, role := range roles.Items {
			issues = append(issues, analyzePolicyRules("Role", role.Namespace, role.Name, role.Rules)...)
		}

		clusterRoles, err := clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list cluster roles: %w", err)
		}
		for _, role := range clusterRoles.Items {
			issues = append(issues, analyzePolicyRules("ClusterRole", "", role.Name, role.Rules)...)
		}

		roleBindings, err := clientset.RbacV1().RoleBindings(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list role bindings: %w", err)
		}
		for _, binding := range roleBindings.Items {
			bindingIssues, err := analyzeRoleBinding(ctx, clientset, binding, "")
			if err != nil {
				return nil, err
			}
			issues = append(issues, bindingIssues...)
		}

		clusterRoleBindings, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list cluster role bindings: %w", err)
		}
		for _, binding := range clusterRoleBindings.Items {
			bindingIssues, err := analyzeClusterRoleBinding(ctx, clientset, binding, "")
			if err != nil {
				return nil, err
			}
			issues = append(issues, bindingIssues...)
		}

		return issues, nil
	}

	roles, err := clientset.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list roles in namespace %s: %w", namespace, err)
	}
	for _, role := range roles.Items {
		issues = append(issues, analyzePolicyRules("Role", role.Namespace, role.Name, role.Rules)...)
	}

	roleBindings, err := clientset.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list role bindings in namespace %s: %w", namespace, err)
	}
	for _, binding := range roleBindings.Items {
		bindingIssues, err := analyzeRoleBinding(ctx, clientset, binding, namespace)
		if err != nil {
			return nil, err
		}
		issues = append(issues, bindingIssues...)
	}

	clusterRoleBindings, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster role bindings: %w", err)
	}
	for _, binding := range clusterRoleBindings.Items {
		bindingIssues, err := analyzeClusterRoleBinding(ctx, clientset, binding, namespace)
		if err != nil {
			return nil, err
		}
		issues = append(issues, bindingIssues...)
	}

	return issues, nil
}

func analyzePolicyRules(kind, namespace, name string, rules []rbacv1.PolicyRule) []RBACIssue {
	issues := []RBACIssue{}
	resourceRef := fmt.Sprintf("%s/%s", kind, name)

	for _, rule := range rules {
		if hasWildcard(rule.Resources) && hasWildcard(rule.Verbs) {
			issues = append(issues, RBACIssue{
				Namespace: namespace,
				Resource:  resourceRef,
				Severity:  "Critical",
				Message:   "RBAC rule grants wildcard resources and verbs",
			})
			continue
		}

		if managesRBAC(rule) {
			issues = append(issues, RBACIssue{
				Namespace: namespace,
				Resource:  resourceRef,
				Severity:  "Critical",
				Message:   "RBAC rule can modify RBAC policies or use bind/escalate privileges",
			})
		}

		if secretAccessSeverity, ok := evaluateSecretAccess(rule); ok {
			issues = append(issues, RBACIssue{
				Namespace: namespace,
				Resource:  resourceRef,
				Severity:  secretAccessSeverity,
				Message:   "RBAC rule grants access to secrets",
			})
		}

		if contains(rule.Verbs, "impersonate") {
			issues = append(issues, RBACIssue{
				Namespace: namespace,
				Resource:  resourceRef,
				Severity:  "Critical",
				Message:   "RBAC rule allows impersonation",
			})
		}
	}

	return issues
}

func analyzeRoleBinding(ctx context.Context, clientset kubernetes.Interface, binding rbacv1.RoleBinding, namespace string) ([]RBACIssue, error) {
	subjects := relevantSubjects(binding.Subjects, namespace)
	if len(subjects) == 0 {
		return nil, nil
	}

	return analyzeRoleRef(ctx, clientset, binding.RoleRef, binding.Namespace, binding.Name, subjects)
}

func analyzeClusterRoleBinding(ctx context.Context, clientset kubernetes.Interface, binding rbacv1.ClusterRoleBinding, namespace string) ([]RBACIssue, error) {
	subjects := relevantSubjects(binding.Subjects, namespace)
	if len(subjects) == 0 {
		return nil, nil
	}

	return analyzeRoleRef(ctx, clientset, binding.RoleRef, "", binding.Name, subjects)
}

func analyzeRoleRef(ctx context.Context, clientset kubernetes.Interface, roleRef rbacv1.RoleRef, bindingNamespace, bindingName string, subjects []string) ([]RBACIssue, error) {
	issues := []RBACIssue{}
	bindingRef := fmt.Sprintf("%s/%s", roleRef.Kind, roleRef.Name)
	subjectRef := strings.Join(subjects, ", ")

	if roleRef.Kind == "ClusterRole" && roleRef.Name == "cluster-admin" {
		issues = append(issues, RBACIssue{
			Namespace: bindingNamespace,
			Resource:  fmt.Sprintf("Binding/%s", bindingName),
			Subject:   subjectRef,
			Severity:  "Critical",
			Message:   fmt.Sprintf("Binding grants cluster-admin via %s", bindingRef),
		})
		return issues, nil
	}

	rules, err := getRoleRules(ctx, clientset, roleRef, bindingNamespace)
	if err != nil {
		return nil, err
	}

	for _, ruleIssue := range analyzePolicyRules(roleRef.Kind, bindingNamespace, roleRef.Name, rules) {
		ruleIssue.Resource = fmt.Sprintf("Binding/%s", bindingName)
		ruleIssue.Subject = subjectRef
		ruleIssue.Message = fmt.Sprintf("%s via %s", ruleIssue.Message, bindingRef)
		issues = append(issues, ruleIssue)
	}

	return issues, nil
}

func getRoleRules(ctx context.Context, clientset kubernetes.Interface, roleRef rbacv1.RoleRef, namespace string) ([]rbacv1.PolicyRule, error) {
	switch roleRef.Kind {
	case "Role":
		role, err := clientset.RbacV1().Roles(namespace).Get(ctx, roleRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get role %s/%s: %w", namespace, roleRef.Name, err)
		}
		return role.Rules, nil
	case "ClusterRole":
		role, err := clientset.RbacV1().ClusterRoles().Get(ctx, roleRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster role %s: %w", roleRef.Name, err)
		}
		return role.Rules, nil
	default:
		return nil, fmt.Errorf("unsupported role ref kind: %s", roleRef.Kind)
	}
}

func relevantSubjects(subjects []rbacv1.Subject, namespace string) []string {
	relevant := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		if namespace != "" && subject.Kind == "ServiceAccount" && subject.Namespace != namespace {
			continue
		}
		relevant = append(relevant, formatSubject(subject))
	}
	return relevant
}

func namespacesToAudit(ctx context.Context, clientset kubernetes.Interface, namespace string) ([]string, error) {
	if namespace != "" {
		return []string{namespace}, nil
	}

	nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		if isSystemNamespace(ns) {
			continue
		}
		namespaces = append(namespaces, ns.Name)
	}

	return namespaces, nil
}

func isSystemNamespace(namespace corev1.Namespace) bool {
	name := namespace.Name
	switch name {
	case "kube-system", "kube-public", "kube-node-lease", "local-path-storage", "cert-manager", "ingress-nginx":
		return true
	default:
		return strings.HasPrefix(name, "kube-")
	}
}

func formatSubject(subject rbacv1.Subject) string {
	if subject.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", subject.Kind, subject.Namespace, subject.Name)
	}
	return fmt.Sprintf("%s/%s", subject.Kind, subject.Name)
}

func hasWildcard(values []string) bool {
	return contains(values, "*")
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func managesRBAC(rule rbacv1.PolicyRule) bool {
	rbacResources := []string{"roles", "clusterroles", "rolebindings", "clusterrolebindings"}
	for _, resource := range rule.Resources {
		if resource == "*" {
			return true
		}
		for _, candidate := range rbacResources {
			if resource == candidate {
				if hasWildcard(rule.Verbs) ||
					contains(rule.Verbs, "create") ||
					contains(rule.Verbs, "update") ||
					contains(rule.Verbs, "patch") ||
					contains(rule.Verbs, "delete") ||
					contains(rule.Verbs, "bind") ||
					contains(rule.Verbs, "escalate") {
					return true
				}
			}
		}
	}

	return contains(rule.Verbs, "bind") || contains(rule.Verbs, "escalate")
}

func evaluateSecretAccess(rule rbacv1.PolicyRule) (string, bool) {
	if !contains(rule.Resources, "secrets") && !contains(rule.Resources, "*") {
		return "", false
	}
	if hasWildcard(rule.Verbs) ||
		contains(rule.Verbs, "create") ||
		contains(rule.Verbs, "update") ||
		contains(rule.Verbs, "patch") ||
		contains(rule.Verbs, "delete") {
		return "Critical", true
	}
	if contains(rule.Verbs, "get") || contains(rule.Verbs, "list") || contains(rule.Verbs, "watch") {
		return "Warning", true
	}
	return "", false
}
