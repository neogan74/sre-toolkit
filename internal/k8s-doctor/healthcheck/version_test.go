package healthcheck

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version string
		want    Version
		wantErr bool
	}{
		{
			version: "v1.28.0",
			want:    Version{Major: 1, Minor: 28, Patch: 0, Extra: ""},
			wantErr: false,
		},
		{
			version: "1.27.4-gke.100",
			want:    Version{Major: 1, Minor: 27, Patch: 4, Extra: "-gke.100"},
			wantErr: false,
		},
		{
			version: "v1.29.0-alpha.1",
			want:    Version{Major: 1, Minor: 29, Patch: 0, Extra: "-alpha.1"},
			wantErr: false,
		},
		{
			version: "invalid",
			want:    Version{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := ParseVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsKubeletCompatible(t *testing.T) {
	v1_28 := Version{Major: 1, Minor: 28, Patch: 0}
	v1_27 := Version{Major: 1, Minor: 27, Patch: 0}
	v1_25 := Version{Major: 1, Minor: 25, Patch: 0}
	v1_24 := Version{Major: 1, Minor: 24, Patch: 0}
	v1_29 := Version{Major: 1, Minor: 29, Patch: 0}

	tests := []struct {
		name    string
		kubelet Version
		api     Version
		want    bool
	}{
		{"same version", v1_28, v1_28, true},
		{"1 minor older", v1_27, v1_28, true},
		{"3 minor older", v1_25, v1_28, true},
		{"4 minor older", v1_24, v1_28, false},
		{"newer", v1_29, v1_28, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsKubeletCompatible(tt.kubelet, tt.api); got != tt.want {
				t.Errorf("IsKubeletCompatible(%v, %v) = %v, want %v", tt.kubelet, tt.api, got, tt.want)
			}
		})
	}
}

func TestGetVersionSkewDescription(t *testing.T) {
	v1_28 := Version{Major: 1, Minor: 28, Patch: 0}
	v1_24 := Version{Major: 1, Minor: 24, Patch: 0}
	v1_29 := Version{Major: 1, Minor: 29, Patch: 0}

	tests := []struct {
		name         string
		compVer      Version
		apiVer       Version
		compName     string
		wantContains string
	}{
		{
			name:         "kubelet too old",
			compVer:      v1_24,
			apiVer:       v1_28,
			compName:     "Kubelet",
			wantContains: "too old",
		},
		{
			name:         "component newer",
			compVer:      v1_29,
			apiVer:       v1_28,
			compName:     "kube-apiserver",
			wantContains: "newer than API server",
		},
		{
			name:         "no issue",
			compVer:      v1_28,
			apiVer:       v1_28,
			compName:     "kube-apiserver",
			wantContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetVersionSkewDescription(tt.compVer, tt.apiVer, tt.compName)
			if tt.wantContains == "" {
				if got != "" {
					t.Errorf("GetVersionSkewDescription() = %q, want empty", got)
				}
			} else {
				if !contains(got, tt.wantContains) {
					t.Errorf("GetVersionSkewDescription() = %q, want to contain %q", got, tt.wantContains)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
