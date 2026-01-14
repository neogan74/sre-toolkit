package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &Config{
				URL: "http://localhost:9090",
			},
			wantErr: false,
		},
		{
			name:    "Missing URL",
			config:  &Config{},
			wantErr: true,
		},
		{
			name: "With Basic Auth",
			config: &Config{
				URL:      "http://localhost:9090",
				Username: "user",
				Password: "password",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, &logger)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.config, client.config)
			}
		})
	}
}

func TestClient_Ping(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		}))
		defer server.Close()

		client, err := NewClient(&Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		err = client.Ping(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client, err := NewClient(&Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		err = client.Ping(context.Background())
		assert.Error(t, err)
	})
}

func TestClient_Query(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up"},"value":[1735689600,"1"]}]}}`))
		}))
		defer server.Close()

		client, err := NewClient(&Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		result, err := client.Query(context.Background(), "up", time.Now())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, model.ValVector, result.Type())
	})

	t.Run("API Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"bad query"}`))
		}))
		defer server.Close()

		client, err := NewClient(&Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		result, err := client.Query(context.Background(), "invalid", time.Now())
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
