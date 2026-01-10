package healthcheck

import (
	"fmt"
	"regexp"
	"strconv"
)

// Version represents a semantic version for Kubernetes components
type Version struct {
	Major int
	Minor int
	Patch int
	Extra string
}

// MaxKubeletSkew defines the maximum allowed minor version skew for kubelet
// Kubernetes policy: kubelet may be up to 3 minor versions older than kube-apiserver
const MaxKubeletSkew = 3

var versionRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(.*)$`)

// ParseVersion parses a Kubernetes version string (e.g., "v1.28.0", "1.27.4-gke.100")
func ParseVersion(v string) (Version, error) {
	matches := versionRegex.FindStringSubmatch(v)
	if len(matches) < 4 {
		return Version{}, fmt.Errorf("invalid version format: %s", v)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Extra: matches[4],
	}, nil
}

// CompareMinor checks the minor version difference between two versions (v1 - v2)
func (v Version) CompareMinor(other Version) int {
	if v.Major != other.Major {
		// For simplicity, we assume Major is always 1 for now
		// but we can return a large value to indicate major version mismatch
		return (v.Major - other.Major) * 1000
	}
	return v.Minor - other.Minor
}

// IsKubeletCompatible checks if a kubelet version is compatible with an API server version
// Policy: kubelet may be up to 3 minor versions older than kube-apiserver, but NOT newer.
func IsKubeletCompatible(kubeletVer, apiVer Version) bool {
	skew := apiVer.CompareMinor(kubeletVer)
	return skew >= 0 && skew <= MaxKubeletSkew
}

// String returns the string representation of the version
func (v Version) String() string {
	res := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Extra != "" {
		res += v.Extra
	}
	return res
}

// GetVersionSkewDescription returns a human-readable description of version skew issues
func GetVersionSkewDescription(componentVer, apiVer Version, componentName string) string {
	skew := apiVer.CompareMinor(componentVer)
	if skew < 0 {
		return fmt.Sprintf("%s version (%s) is newer than API server (%s). This is unsupported.",
			componentName, componentVer, apiVer)
	}
	if skew > MaxKubeletSkew && componentName == "Kubelet" {
		return fmt.Sprintf("%s version (%s) is too old for API server (%s). Max allowed skew is %d minor versions.",
			componentName, componentVer, apiVer, MaxKubeletSkew)
	}
	// For other components, use a more general rule (usually 1-2 versions skew allowed)
	if skew > 2 {
		return fmt.Sprintf("%s version (%s) has significant skew from API server (%s).",
			componentName, componentVer, apiVer)
	}
	return ""
}
