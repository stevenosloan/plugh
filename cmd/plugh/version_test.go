package main

import (
	"strings"
	"testing"
)

func TestPinWarning(t *testing.T) {
	for _, tt := range []struct {
		pin, have string
		want      string // a substring of the warning, or "" for none
	}{
		{"", "v1.0.0", ""},
		{"v1.0.1", devel, ""},
		{"v1.0.1", "v1.0.1", ""},
		{"v1.0.1", "v1.1.0", ""},
		{"v1.0.1", "v1.9.3", ""},
		{"v1.0.1", "v1.0.0", "or later"},
		{"v1.2.0", "v1.2.0-rc.1", "or later"},
		{"v1.0.1", "v2.0.0", "install a v1.x"},
		{"v2.0.0", "v1.9.0", "install a v2.x"},
		{"v0.2.0", "v0.2.5", ""},
		{"v0.2.0", "v0.2.1-0.20261026185959-7fe5cecb1213+dirty", ""}, // a checkout after v0.2.0
		{"v0.2.0", "v0.3.0", "install a v0.2.x"},
		{"v0.2.0", "v0.1.9", "install a v0.2.x"},
	} {
		got := pinWarning(tt.pin, tt.have)
		if tt.want == "" && got != "" || tt.want != "" && !strings.Contains(got, tt.want) {
			t.Errorf("pinWarning(%q, %q) = %q, want %q", tt.pin, tt.have, got, tt.want)
		}
	}
}
