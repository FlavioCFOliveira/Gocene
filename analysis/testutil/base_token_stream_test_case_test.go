// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"testing"
)

// recordingT is a stand-in for *testing.T that captures Errorf and
// Fatalf calls so the helper-under-test can be exercised in both
// passing and failing modes without aborting the outer test.
type recordingT struct {
	*testing.T
	errors []string
	fatals []string
}

func (r *recordingT) Helper() {}

func (r *recordingT) Errorf(format string, args ...any) {
	r.errors = append(r.errors, sprintf(format, args...))
}

// Fatalf records the failure and aborts the current goroutine via
// runtime.Goexit (mirroring testing.T.Fatalf). The caller must
// dispatch the helper invocation through runHelper so the goroutine
// boundary is preserved.
func (r *recordingT) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, sprintf(format, args...))
	// Mirror testing.T.Fatalf by stopping execution of the caller.
	panic(fatalSentinel{})
}

type fatalSentinel struct{}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func runHelper(fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fatalSentinel); ok {
				panicked = true
				return
			}
			panic(r)
		}
	}()
	fn()
	return false
}

// TestAssertTokenStreamContents_PassMinimal exercises the simplest
// happy path: term text only.
func TestAssertTokenStreamContents_PassMinimal(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStream(
		NewToken("alpha", 0, 5),
		NewToken("beta", 6, 10),
	)

	AssertTokenStreamContentsSimple(t, ts, []string{"alpha", "beta"})
}

// TestAssertTokenStreamContents_PassFull covers every assertable
// attribute on a canned stream.
func TestAssertTokenStreamContents_PassFull(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStreamWithFinal(0, 13,
		NewTokenWithPosIncAndLength("the", 1, 0, 3, 1).WithType("word").WithFlags(0),
		NewTokenWithPosIncAndLength("quick", 1, 4, 9, 1).WithType("word").WithFlags(0),
		NewTokenWithPosIncAndLength("fox", 1, 10, 13, 1).WithType("word").WithFlags(0),
	)

	finalOffset := 13
	finalPosInc := 0
	AssertTokenStreamContents(t, ts, TokenStreamExpectations{
		Terms:                  []string{"the", "quick", "fox"},
		StartOffsets:           []int{0, 4, 10},
		EndOffsets:             []int{3, 9, 13},
		Types:                  []string{"word", "word", "word"},
		PositionIncrements:     []int{1, 1, 1},
		PositionLengths:        []int{1, 1, 1},
		FinalOffset:            &finalOffset,
		FinalPositionIncrement: &finalPosInc,
		Flags:                  []int{0, 0, 0},
	})
}

// TestAssertTokenStreamContents_FailWrongTerm verifies that a wrong
// expected term triggers Errorf without aborting the test.
func TestAssertTokenStreamContents_FailWrongTerm(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStream(
		NewToken("alpha", 0, 5),
		NewToken("beta", 6, 10),
	)

	rt := &recordingT{T: t}
	panicked := runHelper(func() {
		AssertTokenStreamContents(rt, ts, TokenStreamExpectations{
			Terms: []string{"alpha", "GAMMA"},
		})
	})
	if panicked {
		t.Fatalf("expected Errorf (non-fatal), but a Fatalf was raised: %v", rt.fatals)
	}
	if len(rt.errors) == 0 {
		t.Fatalf("expected at least one Errorf for wrong term, got none")
	}
}

// TestAssertTokenStreamContents_FailExtraToken verifies that a
// stream emitting more tokens than expected fails (Errorf, not
// Fatalf — Lucene's reference also continues).
func TestAssertTokenStreamContents_FailExtraToken(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStream(
		NewToken("alpha", 0, 5),
		NewToken("beta", 6, 10),
	)

	rt := &recordingT{T: t}
	panicked := runHelper(func() {
		AssertTokenStreamContentsSimple(rt, ts, []string{"alpha"})
	})
	if panicked {
		t.Fatalf("expected non-fatal Errorf, got fatal: %v", rt.fatals)
	}
	if len(rt.errors) == 0 {
		t.Fatalf("expected Errorf for extra token, got none")
	}
}

// TestAssertTokenStreamContents_FailMissingToken verifies that a
// stream emitting fewer tokens than expected fails with Fatalf, since
// the remaining assertions cannot run.
func TestAssertTokenStreamContents_FailMissingToken(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStream(
		NewToken("alpha", 0, 5),
	)

	rt := &recordingT{T: t}
	panicked := runHelper(func() {
		AssertTokenStreamContentsSimple(rt, ts, []string{"alpha", "beta"})
	})
	if !panicked {
		t.Fatalf("expected Fatalf for stream exhausted early; got errors=%v", rt.errors)
	}
	if len(rt.fatals) == 0 {
		t.Fatalf("expected Fatalf message, got none")
	}
}

// TestAssertTokenStreamContents_FailOffsetMismatch verifies that
// per-token offset mismatches are reported as Errorf.
func TestAssertTokenStreamContents_FailOffsetMismatch(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStream(NewToken("hello", 0, 5))

	rt := &recordingT{T: t}
	panicked := runHelper(func() {
		AssertTokenStreamContentsOffsets(rt, ts, []string{"hello"}, []int{0}, []int{99})
	})
	if panicked {
		t.Fatalf("expected Errorf for offset mismatch, got Fatalf: %v", rt.fatals)
	}
	if len(rt.errors) == 0 {
		t.Fatalf("expected at least one Errorf for endOffset mismatch")
	}
}

// TestAssertTokenStreamContents_FailFinalOffset verifies that a
// wrong finalOffset is caught by End()-time checks.
func TestAssertTokenStreamContents_FailFinalOffset(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStreamWithFinal(0, 10, NewToken("a", 0, 1))

	rt := &recordingT{T: t}
	panicked := runHelper(func() {
		AssertTokenStreamContentsOffsetsFinal(rt, ts, []string{"a"}, []int{0}, []int{1}, 99)
	})
	if panicked {
		t.Fatalf("unexpected Fatalf: %v", rt.fatals)
	}
	if len(rt.errors) == 0 {
		t.Fatalf("expected Errorf for finalOffset mismatch")
	}
}

// TestAssertTokenStreamContents_VariantsCompose verifies that the
// short helper variants delegate to the canonical entry point
// consistently.
func TestAssertTokenStreamContents_VariantsCompose(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStream(
		NewToken("alpha", 0, 5),
		NewToken("beta", 6, 10),
	)
	AssertTokenStreamContentsTypes(t, ts, []string{"alpha", "beta"}, []string{"word", "word"})

	ts2 := NewCannedTokenStream(
		NewTokenWithPosInc("alpha", 1, 0, 5),
		NewTokenWithPosInc("beta", 1, 6, 10),
	)
	AssertTokenStreamContentsPosInc(t, ts2, []string{"alpha", "beta"}, []int{1, 1})

	ts3 := NewCannedTokenStream(
		NewToken("alpha", 0, 5),
		NewToken("beta", 6, 10),
	)
	AssertTokenStreamContentsOffsets(t, ts3, []string{"alpha", "beta"}, []int{0, 6}, []int{5, 10})
}

// TestAssertTokenStreamContents_LengthMismatchFatal verifies that a
// caller-supplied slice with the wrong length fatals out before any
// stream traversal happens.
func TestAssertTokenStreamContents_LengthMismatchFatal(t *testing.T) {
	t.Parallel()

	ts := NewCannedTokenStream(NewToken("a", 0, 1))

	rt := &recordingT{T: t}
	panicked := runHelper(func() {
		AssertTokenStreamContents(rt, ts, TokenStreamExpectations{
			Terms:        []string{"a"},
			StartOffsets: []int{0, 99}, // wrong length
		})
	})
	if !panicked {
		t.Fatalf("expected Fatalf for length mismatch, got errors=%v", rt.errors)
	}
}
