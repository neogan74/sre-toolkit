package healthcheck

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckEvents(t *testing.T) {
	tests := []struct {
		name          string
		events        []corev1.Event
		wantWarning   int
		wantError     int
		wantTotalItem int
	}{
		{
			name: "warning and error events",
			events: []corev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "e1", Namespace: "default"},
					Type:       corev1.EventTypeWarning,
					Reason:     "FailedMount",
					Message:    "failed to mount volume",
					Count:      1,
					InvolvedObject: corev1.ObjectReference{
						Kind: "Pod",
						Name: "p1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "e2", Namespace: "default"},
					Type:       "Error",
					Reason:     "FailedCreate",
					Message:    "failed to create pod",
					Count:      5,
					InvolvedObject: corev1.ObjectReference{
						Kind: "Deployment",
						Name: "d1",
					},
				},
			},
			wantWarning:   1,
			wantError:     1,
			wantTotalItem: 2,
		},
		{
			name:          "no events",
			events:        []corev1.Event{},
			wantWarning:   0,
			wantError:     0,
			wantTotalItem: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			for i := range tt.events {
				objs = append(objs, &tt.events[i])
			}
			clientset := fake.NewSimpleClientset(objs...)

			got, err := CheckEvents(context.Background(), clientset, "")
			if err != nil {
				t.Fatalf("CheckEvents() error = %v", err)
			}

			if got.TotalWarning != tt.wantWarning {
				t.Errorf("CheckEvents() totalWarning = %v, want %v", got.TotalWarning, tt.wantWarning)
			}
			if got.TotalError != tt.wantError {
				t.Errorf("CheckEvents() totalError = %v, want %v", got.TotalError, tt.wantError)
			}
			if len(got.Events) != tt.wantTotalItem {
				t.Errorf("CheckEvents() total items = %v, want %v", len(got.Events), tt.wantTotalItem)
			}
		})
	}
}
