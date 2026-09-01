package vectorhighlight

import (
	"testing"
)

func TestSimpleBoundaryScanner_FindStartOffset(t *testing.T) {
	tests := []struct {
		name       string
		maxScan    int
		boundaries []rune
		text       string
		start      int
		want       int
	}{
		{"Normal found", 20, nil, "Hello world", 6, 6},       // ' ' at index 5, return 6
		{"Start of text", 20, nil, "Hello world", 1, 0},     // no boundary before 1, but hits offset 0
		{"No boundary within maxScan", 2, nil, "Hello world", 6, 6}, // ' ' at 5, but start=6, maxScan=2 -> scans 5, 4. 'o', 'l' not boundaries. returns start.
		{"Boundary at exactly maxScan", 2, nil, "Hello world", 7, 6}, // ' ' at 5. start=7. scans 6 ('w'), 5 (' '). found at 5 -> return 6.
		{"Empty text", 20, nil, "", 0, 0},
		{"Offset too large", 20, nil, "Hello", 10, 10},
		{"Offset too small", 20, nil, "Hello", 0, 0},
		{"Custom boundaries", 20, []rune{'-'}, "foo-bar", 5, 4}, // '-' at index 3, return 4
		{"No custom boundary", 20, []rune{'-'}, "foobar", 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSimpleBoundaryScanner(tt.maxScan, tt.boundaries)
			if got := s.FindStartOffset(tt.text, tt.start); got != tt.want {
				t.Errorf("FindStartOffset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleBoundaryScanner_FindEndOffset(t *testing.T) {
	tests := []struct {
		name       string
		maxScan    int
		boundaries []rune
		text       string
		start      int
		want       int
	}{
		{"Normal found", 20, nil, "Hello world", 5, 5},       // ' ' at index 5, return 5
		{"End of text", 20, nil, "Hello world", 11, 11},     // start=11, len=11, return 11
		{"No boundary within maxScan", 2, nil, "Hello world", 1, 1}, // ' ' at 5. start=1. scans 1, 2. 'e', 'l'. return start.
		{"Boundary at exactly maxScan", 5, nil, "Hello world", 1, 5}, // ' ' at 5. start=1. scans 1, 2, 3, 4, 5. found at 5 -> return 5.
		{"Empty text", 20, nil, "", 0, 0},
		{"Offset too large", 20, nil, "Hello", 10, 10},
		{"Offset too small", 20, nil, "Hello", -1, -1},
		{"Custom boundaries", 20, []rune{'-'}, "foo-bar", 1, 3}, // '-' at index 3, return 3
		{"No custom boundary", 20, []rune{'-'}, "foobar", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSimpleBoundaryScanner(tt.maxScan, tt.boundaries)
			if got := s.FindEndOffset(tt.text, tt.start); got != tt.want {
				t.Errorf("FindEndOffset() = %v, want %v", got, tt.want)
			}
		})
	}
}
