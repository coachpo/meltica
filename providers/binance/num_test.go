package binance

import "testing"

func TestScaleFromStep(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"0.01000000", 2},
		{"0.0001", 4},
		{"1", 0},
		{"0.100", 1},
	}
	for _, c := range cases {
		if got := scaleFromStep(c.in); got != c.want {
			t.Fatalf("scaleFromStep(%q)=%d want %d", c.in, got, c.want)
		}
	}
}
