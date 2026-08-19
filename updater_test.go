package main

import "testing"

func TestVersionGreater(t *testing.T) {
	tests := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"0.9.1", "0.9.0", true},
		{"0.10.0", "0.9.9", true},
		{"1.0.0", "0.9.9", true},
		{"0.9.0", "0.9.0", false},
		{"0.8.9", "0.9.0", false},
		{"0.9.0", "0.9.1", false},
	}
	for _, test := range tests {
		if got := versionGreater(test.candidate, test.current); got != test.want {
			t.Errorf("versionGreater(%q, %q) = %t, want %t", test.candidate, test.current, got, test.want)
		}
	}
}
