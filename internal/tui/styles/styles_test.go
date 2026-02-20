package styles

import "testing"

func TestFormatNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1001, "1,001"},
		{9999, "9,999"},
		{10000, "10,000"},
		{100000, "100,000"},
		{1000000, "1,000,000"},
		{1234567, "1,234,567"},
		{9234567, "9,234,567"},
		{1000000000, "1,000,000,000"},
		// Negative numbers
		{-1, "-1"},
		{-1000, "-1,000"},
		{-1234567, "-1,234,567"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatNumber(tt.n)
			if result != tt.expected {
				t.Errorf("FormatNumber(%d) = %q, want %q", tt.n, result, tt.expected)
			}
		})
	}
}
