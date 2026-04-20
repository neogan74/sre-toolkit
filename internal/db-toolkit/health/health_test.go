package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorstStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []Check
		want   Status
	}{
		{
			name:   "empty returns OK",
			checks: nil,
			want:   StatusOK,
		},
		{
			name: "all OK",
			checks: []Check{
				{Status: StatusOK},
				{Status: StatusOK},
			},
			want: StatusOK,
		},
		{
			name: "warning present",
			checks: []Check{
				{Status: StatusOK},
				{Status: StatusWarning},
			},
			want: StatusWarning,
		},
		{
			name: "critical present",
			checks: []Check{
				{Status: StatusOK},
				{Status: StatusWarning},
				{Status: StatusCritical},
			},
			want: StatusCritical,
		},
		{
			name: "critical dominates warning",
			checks: []Check{
				{Status: StatusWarning},
				{Status: StatusCritical},
				{Status: StatusWarning},
			},
			want: StatusCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worstStatus(tt.checks)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, Status("OK"), StatusOK)
	assert.Equal(t, Status("WARNING"), StatusWarning)
	assert.Equal(t, Status("CRITICAL"), StatusCritical)
}
