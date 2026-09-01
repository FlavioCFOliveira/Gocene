package main

import (
	"fmt"
	"testing"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/index"
)

func main() {
	// Manual test for ExactPhraseMatcher
	p1 := &search.MockPostingsEnum{DocID: 0, Freq: 2, Positions: []int{2, 5}}
	p2 := &search.MockPostingsEnum{DocID: 0, Freq: 2, Positions: []int{3, 8}}

	matcher := search.NewExactPhraseMatcher([]struct {
		postings index.PostingsEnum
		offset   int
	}{
		{postings: p1, offset: 0},
		{postings: p2, offset: 1},
	}, search.ScoreModeTopScores, nil, 1.0)

	matcher.ResetPositions()

	ok, _ := matcher.NextMatch()
	fmt.Printf("Match 1: %v, Start: %d, End: %d\n", ok, matcher.StartPosition(), matcher.EndPosition())

	ok, _ = matcher.NextMatch()
	fmt.Printf("Match 2: %v\n", ok)
}
