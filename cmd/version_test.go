package cmd

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected int
	}{
		{"v0.5.0", "v0.5.1", -1},
		{"v0.5.1", "v0.5.0", 1},
		{"v0.5.0", "v0.5.0", 0},
		{"0.5.0", "v0.5.1", -1},
		{"v0.6.0", "v0.5.9", 1},
		{"v1.0.0", "v0.9.9", 1},
		{"dev", "v0.5.0", -1},
	}

	for _, tt := range tests {
		res := compareVersions(tt.current, tt.latest)
		if (tt.expected < 0 && res >= 0) || (tt.expected > 0 && res <= 0) || (tt.expected == 0 && res != 0) {
			t.Errorf("compareVersions(%q, %q) = %d, expected sign %d", tt.current, tt.latest, res, tt.expected)
		}
	}
}
