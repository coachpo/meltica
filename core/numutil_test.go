package core

import "testing"

func TestScaleFromStep(t *testing.T) {
	tests := []struct {
		step string
		want int
	}{
		{"", 0},
		{"1", 0},
		{"0.0100", 2},
		{"1e-3", 3},
		{"2.500e-3", 4},
		{"5e2", 0},
		{"  1E-6 ", 6},
	}

	for _, tt := range tests {
		if got := ScaleFromStep(tt.step); got != tt.want {
			t.Errorf("ScaleFromStep(%q) = %d, want %d", tt.step, got, tt.want)
		}
	}
}
