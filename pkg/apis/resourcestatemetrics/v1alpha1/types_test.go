package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceMetricsMonitorStatus_Set(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		condition metav1.Condition
		want      ResourceMetricsMonitorStatus
	}{
		{
			name: "Processed condition with truthy status",
			condition: metav1.Condition{
				Type:   "Processed",
				Status: metav1.ConditionTrue,
			},
			want: ResourceMetricsMonitorStatus{
				Conditions: []metav1.Condition{
					{
						Type:    "Processed",
						Status:  metav1.ConditionTrue,
						Reason:  "EventHandlerSucceeded",
						Message: "Resource configuration has been processed successfully",
					},
				},
			},
		},
		{
			name: "Failed condition with truthy status",
			condition: metav1.Condition{
				Type:   "Failed",
				Status: metav1.ConditionTrue,
			},
			want: ResourceMetricsMonitorStatus{
				Conditions: []metav1.Condition{
					{
						Type:    "Failed",
						Status:  metav1.ConditionTrue,
						Reason:  "EventHandlerFailed",
						Message: "Resource failed to process",
					},
				},
			},
		},
		{
			name: "Processed condition with false status",
			condition: metav1.Condition{
				Type:   "Processed",
				Status: metav1.ConditionFalse,
			},
			want: ResourceMetricsMonitorStatus{
				Conditions: []metav1.Condition{
					{
						Type:    "Processed",
						Status:  metav1.ConditionFalse,
						Reason:  "EventHandlerRunning",
						Message: "Resource configuration is yet to be processed",
					},
				},
			},
		},
		{
			name: "Failed condition with false status",
			condition: metav1.Condition{
				Type:   "Failed",
				Status: metav1.ConditionFalse,
			},
			want: ResourceMetricsMonitorStatus{
				Conditions: []metav1.Condition{
					{
						Type:    "Failed",
						Status:  metav1.ConditionFalse,
						Reason:  "N/A",
						Message: "N/A",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status := ResourceMetricsMonitorStatus{}
			status.Set(&ResourceMetricsMonitor{}, tt.condition)

			// Set transient fields.
			status.Conditions[0].LastTransitionTime = metav1.Time{}
			if !cmp.Equal(status, tt.want) {
				t.Errorf("%s", cmp.Diff(status, tt.want))
			}
		})
	}
}
