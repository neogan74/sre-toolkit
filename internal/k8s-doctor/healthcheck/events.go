package healthcheck

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EventStatus represents a summary of filtered events
type EventStatus struct {
	TotalWarning int
	TotalError   int
	Events       []FilteredEvent
}

// FilteredEvent represents a relevant cluster event
type FilteredEvent struct {
	Namespace string
	Name      string
	Type      string
	Reason    string
	Message   string
	Count     int32
	Object    string
}

// CheckEvents collects and filters cluster events
func CheckEvents(ctx context.Context, clientset kubernetes.Interface, namespace string) (*EventStatus, error) {
	opts := metav1.ListOptions{
		FieldSelector: "type!=Normal",
	}

	events, err := clientset.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	status := &EventStatus{
		Events: []FilteredEvent{},
	}

	for _, event := range events.Items {
		fe := FilteredEvent{
			Namespace: event.Namespace,
			Name:      event.Name,
			Type:      event.Type,
			Reason:    event.Reason,
			Message:   event.Message,
			Count:     event.Count,
			Object:    fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
		}

		if event.Type == corev1.EventTypeWarning {
			status.TotalWarning++
		} else {
			status.TotalError++
		}

		status.Events = append(status.Events, fe)
	}

	return status, nil
}
