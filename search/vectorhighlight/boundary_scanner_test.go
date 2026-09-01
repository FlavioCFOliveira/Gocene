package vectorhighlight

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/highlight"
)

func TestBreakIteratorBoundaryScanner(t *testing.T) {
	bi := highlight.NewSentenceBreakIterator()
	scanner := NewBreakIteratorBoundaryScanner(bi)

	tests := []struct {
		name       string
		text       string
		start      int
		wantStart  int
		wantEnd    int
	}{
		{
			name:       "Simple word",
			text:       "Hello world",
			start:      1,
			wantStart:  0,
			wantEnd:    5,
		},
		{
			name:       "Middle of second word",
			text:       "Hello world",
			start:      7,
			wantStart:  6,
			wantEnd:    11,
		},
		{
			name:       "At boundary",
			text:       "Hello world",
			start:      5,
			wantStart:  0,
			wantEnd:    6,
		},
		{
			name:       "Start of string",
			text:       "Hello world",
			start:      0,
			wantStart:  0,
			wantEnd:    5,
		},
		{
			name:       "End of string",
			text:       "Hello world",
			start:      11,
			wantStart:  6,
			wantEnd:    11,
		},
		{
			name:       "With punctuation",
			text:       "Hello, world!",
			start:      1,
			wantStart:  0,
			wantEnd:    5,
		},
		{
			name:       "Punctuation boundary",
			text:       "Hello, world!",
			start:      5,
			wantStart:  0,
			wantEnd:    7,
		},
		{
			name:       "Multi-byte characters",
			text:       "Привет мир",
			start:      2, // 'р'
			wantStart:  0,
			wantEnd:    12,
		},
		{
			name:       "Multi-byte second word",
			text:       "Привет мир",
			start:      13, // 'и'
			wantStart:  12,
			wantEnd:    19,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart := scanner.FindStartOffset(tt.text, tt.start)
			if gotStart != tt.wantStart {
				t.Errorf("FindStartOffset(%q, %d) = %d; want %d", tt.text, tt.start, gotStart, tt.wantStart)
			}

			gotEnd := scanner.FindEndOffset(tt.text, tt.start)
			if gotEnd != tt.wantEnd {
				t.Errorf("FindEndOffset(%q, %d) = %d; want %d", tt.text, tt.start, gotEnd, tt.wantEnd)
			}
		})
	}
}
