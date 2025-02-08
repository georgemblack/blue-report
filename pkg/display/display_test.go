package display

import "testing"

func TestFormatCount(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		output string
	}{
		{
			name:   "under 1000",
			count:  500,
			output: "500",
		},
		{
			name:   "between 1000 and 10000, rounding down",
			count:  1549,
			output: "1.5k",
		},
		{
			name:   "between 1000 and 10000, rounding up",
			count:  1549,
			output: "1.5k",
		},
		{
			name:   "over 10000, no rounding",
			count:  15999,
			output: "15k",
		},
		{
			name:   "over 10000",
			count:  16000,
			output: "16k",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCount(tt.count)
			if result != tt.output {
				t.Errorf("expected %s, got %s", tt.output, result)
			}
		})
	}
}
