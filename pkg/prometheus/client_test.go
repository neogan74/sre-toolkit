package prometheus

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAPI is a mock for v1.API
type MockAPI struct {
	mock.Mock
}

func (m *MockAPI) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.AlertsResult), args.Error(1)
}

func (m *MockAPI) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.AlertManagersResult), args.Error(1)
}

func (m *MockAPI) CleanTombstones(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAPI) Config(ctx context.Context) (v1.ConfigResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.ConfigResult), args.Error(1)
}

func (m *MockAPI) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	args := m.Called(ctx, matches, startTime, endTime)
	return args.Error(0)
}

func (m *MockAPI) Flags(ctx context.Context) (v1.FlagsResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.FlagsResult), args.Error(1)
}

func (m *MockAPI) LabelNames(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) ([]string, v1.Warnings, error) {
	args := m.Called(ctx, matches, startTime, endTime)
	return args.Get(0).([]string), args.Get(1).(v1.Warnings), args.Error(2)
}

func (m *MockAPI) LabelValues(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
	args := m.Called(ctx, label, matches, startTime, endTime)
	var v model.LabelValues
	if args.Get(0) != nil {
		v = args.Get(0).(model.LabelValues)
	}
	var w v1.Warnings
	if args.Get(1) != nil {
		w = args.Get(1).(v1.Warnings)
	}
	return v, w, args.Error(2)
}

func (m *MockAPI) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	args := m.Called(ctx, query, ts)
	var v model.Value
	if args.Get(0) != nil {
		v = args.Get(0).(model.Value)
	}
	var w v1.Warnings
	if args.Get(1) != nil {
		w = args.Get(1).(v1.Warnings)
	}
	return v, w, args.Error(2)
}

func (m *MockAPI) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	args := m.Called(ctx, query, r)
	var v model.Value
	if args.Get(0) != nil {
		v = args.Get(0).(model.Value)
	}
	var w v1.Warnings
	if args.Get(1) != nil {
		w = args.Get(1).(v1.Warnings)
	}
	return v, w, args.Error(2)
}

func (m *MockAPI) QueryExemplars(ctx context.Context, query string, startTime time.Time, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
	args := m.Called(ctx, query, startTime, endTime)
	return args.Get(0).([]v1.ExemplarQueryResult), args.Error(1)
}

func (m *MockAPI) Buildinfo(ctx context.Context) (v1.BuildinfoResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.BuildinfoResult), args.Error(1)
}

func (m *MockAPI) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.RuntimeinfoResult), args.Error(1)
}

func (m *MockAPI) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
	args := m.Called(ctx, matches, startTime, endTime)
	return args.Get(0).([]model.LabelSet), args.Get(1).(v1.Warnings), args.Error(2)
}

func (m *MockAPI) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	args := m.Called(ctx, skipHead)
	return args.Get(0).(v1.SnapshotResult), args.Error(1)
}

func (m *MockAPI) Rules(ctx context.Context) (v1.RulesResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.RulesResult), args.Error(1)
}

func (m *MockAPI) Targets(ctx context.Context) (v1.TargetsResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.TargetsResult), args.Error(1)
}

func (m *MockAPI) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]v1.MetricMetadata, error) {
	args := m.Called(ctx, matchTarget, metric, limit)
	return args.Get(0).([]v1.MetricMetadata), args.Error(1)
}

func (m *MockAPI) Metadata(ctx context.Context, metric string, limit string) (map[string][]v1.Metadata, error) {
	args := m.Called(ctx, metric, limit)
	return args.Get(0).(map[string][]v1.Metadata), args.Error(1)
}

func (m *MockAPI) TSDB(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.TSDBResult), args.Error(1)
}

func (m *MockAPI) WalReplay(ctx context.Context) (v1.WalReplayStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(v1.WalReplayStatus), args.Error(1)
}

func TestQueryRange(t *testing.T) {
	logger := zerolog.Nop()
	mockAPI := new(MockAPI)
	client := &Client{
		api:    mockAPI,
		logger: &logger,
	}

	ctx := context.Background()
	query := "up"
	r := v1.Range{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
		Step:  1 * time.Minute,
	}

	t.Run("success", func(t *testing.T) {
		expectedValue := model.Matrix{}
		mockAPI.On("QueryRange", ctx, query, r).Return(expectedValue, v1.Warnings(nil), nil).Once()

		result, err := client.QueryRange(ctx, query, r)
		assert.NoError(t, err)
		assert.Equal(t, expectedValue, result)
	})

	t.Run("error", func(t *testing.T) {
		mockAPI.On("QueryRange", ctx, query, r).Return(nil, v1.Warnings(nil), errors.New("api error")).Once()

		result, err := client.QueryRange(ctx, query, r)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "range query failed")
	})

	t.Run("warnings", func(t *testing.T) {
		expectedValue := model.Matrix{}
		warnings := v1.Warnings{"some warning"}
		mockAPI.On("QueryRange", ctx, query, r).Return(expectedValue, warnings, nil).Once()

		result, err := client.QueryRange(ctx, query, r)
		assert.NoError(t, err)
		assert.Equal(t, expectedValue, result)
	})
}

func TestLabelValues(t *testing.T) {
	logger := zerolog.Nop()
	mockAPI := new(MockAPI)
	client := &Client{
		api:    mockAPI,
		logger: &logger,
	}

	ctx := context.Background()
	label := "instance"

	t.Run("success", func(t *testing.T) {
		expectedValues := model.LabelValues{"node1", "node2"}
		// LabelValues uses time.Now() internally, so we match arguments with mock.Anything
		mockAPI.On("LabelValues", ctx, label, []string(nil), mock.Anything, mock.Anything).
			Return(expectedValues, v1.Warnings(nil), nil).Once()

		values, err := client.LabelValues(ctx, label)
		assert.NoError(t, err)
		assert.Equal(t, expectedValues, values)
	})

	t.Run("error", func(t *testing.T) {
		mockAPI.On("LabelValues", ctx, label, []string(nil), mock.Anything, mock.Anything).
			Return(model.LabelValues(nil), v1.Warnings(nil), errors.New("api error")).Once()

		values, err := client.LabelValues(ctx, label)
		assert.Error(t, err)
		assert.Nil(t, values)
		assert.Contains(t, err.Error(), "failed to get label values")
	})

	t.Run("warnings", func(t *testing.T) {
		expectedValues := model.LabelValues{"node1"}
		warnings := v1.Warnings{"some warning"}
		mockAPI.On("LabelValues", ctx, label, []string(nil), mock.Anything, mock.Anything).
			Return(expectedValues, warnings, nil).Once()

		values, err := client.LabelValues(ctx, label)
		assert.NoError(t, err)
		assert.Equal(t, expectedValues, values)
	})
}

func TestBuildInfo(t *testing.T) {
	logger := zerolog.Nop()
	mockAPI := new(MockAPI)
	client := &Client{
		api:    mockAPI,
		logger: &logger,
	}

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expectedInfo := v1.BuildinfoResult{
			Version: "2.45.0",
		}
		mockAPI.On("Buildinfo", ctx).Return(expectedInfo, nil).Once()

		info, err := client.BuildInfo(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedInfo, info)
	})

	t.Run("error", func(t *testing.T) {
		mockAPI.On("Buildinfo", ctx).Return(v1.BuildinfoResult{}, errors.New("api error")).Once()

		info, err := client.BuildInfo(ctx)
		assert.Error(t, err)
		assert.Empty(t, info.Version)
	})
}

func TestClient_Auth(t *testing.T) {
	// Custom RoundTripper to capture the request
	type captureRoundTripper struct {
		req *http.Request
	}

	captured := &captureRoundTripper{}
	mockRT := mock.Mock{}
	mockRT.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
		captured.req = args.Get(0).(*http.Request)
	}).Return(&http.Response{StatusCode: 200}, nil)

	// Create the auth round tripper
	authRT := &basicAuthRoundTripper{
		username: "user",
		password: "password",
		next:     &mockTransport{mock: &mockRT},
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err := authRT.RoundTrip(req)

	assert.NoError(t, err)
	u, p, ok := captured.req.BasicAuth()
	assert.True(t, ok)
	assert.Equal(t, "user", u)
	assert.Equal(t, "password", p)
}

type mockTransport struct {
	mock *mock.Mock
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.mock.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}
