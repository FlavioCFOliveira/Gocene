// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TestCannedTokenStream_Empty covers the boundary case of zero canned
// tokens: the first IncrementToken must return (false, nil) and End()
// must still apply the default trailing finalPosInc=0 / finalOffset=0.
func TestCannedTokenStream_Empty(t *testing.T) {
	t.Parallel()

	cts := NewCannedTokenStream()

	ok, err := cts.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken on empty stream: unexpected error %v", err)
	}
	if ok {
		t.Fatalf("IncrementToken on empty stream: got ok=true, want false")
	}

	if err := cts.End(); err != nil {
		t.Fatalf("End on empty stream: unexpected error %v", err)
	}
	if got := cts.PositionIncrementAttribute().GetPositionIncrement(); got != 0 {
		t.Errorf("finalPosInc default: got %d, want 0", got)
	}
	if start, end := cts.OffsetAttribute().StartOffset(), cts.OffsetAttribute().EndOffset(); start != 0 || end != 0 {
		t.Errorf("final offset default: got (%d,%d), want (0,0)", start, end)
	}
}

// TestCannedTokenStream_Single covers a single-token stream and
// verifies that Lucene-faithful attribute defaults (type="word",
// posInc=1, posLen=1) are applied when the Token did not set them.
func TestCannedTokenStream_Single(t *testing.T) {
	t.Parallel()

	cts := NewCannedTokenStream(NewToken("hello", 0, 5))

	ok, err := cts.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken: unexpected error %v", err)
	}
	if !ok {
		t.Fatalf("IncrementToken: got ok=false on first call, want true")
	}

	if got := cts.CharTermAttribute().String(); got != "hello" {
		t.Errorf("term text: got %q, want %q", got, "hello")
	}
	if start, end := cts.OffsetAttribute().StartOffset(), cts.OffsetAttribute().EndOffset(); start != 0 || end != 5 {
		t.Errorf("offsets: got (%d,%d), want (0,5)", start, end)
	}
	if got := cts.TypeAttribute().GetType(); got != analysis.DefaultTokenType {
		t.Errorf("type default: got %q, want %q", got, analysis.DefaultTokenType)
	}
	if got := cts.PositionIncrementAttribute().GetPositionIncrement(); got != 1 {
		t.Errorf("posInc default: got %d, want 1", got)
	}
	if got := cts.PositionLengthAttribute().GetPositionLength(); got != 1 {
		t.Errorf("posLen default: got %d, want 1", got)
	}
	if got := cts.FlagsAttribute().GetFlags(); got != 0 {
		t.Errorf("flags default: got %d, want 0", got)
	}
	if got := cts.PayloadAttribute().GetPayload(); got != nil {
		t.Errorf("payload default: got %v, want nil", got)
	}

	// Exhausted.
	ok, err = cts.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken (exhausted): unexpected error %v", err)
	}
	if ok {
		t.Errorf("IncrementToken (exhausted): got ok=true, want false")
	}
}

// TestCannedTokenStream_MultiPosition covers a stream with explicit
// position-increment / position-length values and verifies that each
// token's attributes are emitted in order.
func TestCannedTokenStream_MultiPosition(t *testing.T) {
	t.Parallel()

	tokens := []Token{
		NewTokenWithPosIncAndLength("the", 1, 0, 3, 1),
		NewTokenWithPosIncAndLength("quick", 1, 4, 9, 1),
		// Synonym-style: same position as previous, span 2 tokens.
		NewTokenWithPosIncAndLength("brown-fox", 0, 10, 19, 2),
		NewTokenWithPosIncAndLength("brown", 1, 10, 15, 1),
		NewTokenWithPosIncAndLength("fox", 1, 16, 19, 1),
	}
	cts := NewCannedTokenStream(tokens...)

	want := []struct {
		text       string
		start, end int
		posInc     int
		posLen     int
	}{
		{"the", 0, 3, 1, 1},
		{"quick", 4, 9, 1, 1},
		{"brown-fox", 10, 19, 0, 2},
		{"brown", 10, 15, 1, 1},
		{"fox", 16, 19, 1, 1},
	}

	for i, w := range want {
		ok, err := cts.IncrementToken()
		if err != nil {
			t.Fatalf("token #%d: IncrementToken error %v", i, err)
		}
		if !ok {
			t.Fatalf("token #%d: IncrementToken got ok=false, want true", i)
		}
		if got := cts.CharTermAttribute().String(); got != w.text {
			t.Errorf("token #%d text: got %q, want %q", i, got, w.text)
		}
		if start, end := cts.OffsetAttribute().StartOffset(), cts.OffsetAttribute().EndOffset(); start != w.start || end != w.end {
			t.Errorf("token #%d offsets: got (%d,%d), want (%d,%d)", i, start, end, w.start, w.end)
		}
		if got := cts.PositionIncrementAttribute().GetPositionIncrement(); got != w.posInc {
			t.Errorf("token #%d posInc: got %d, want %d", i, got, w.posInc)
		}
		if got := cts.PositionLengthAttribute().GetPositionLength(); got != w.posLen {
			t.Errorf("token #%d posLen: got %d, want %d", i, got, w.posLen)
		}
	}

	ok, err := cts.IncrementToken()
	if err != nil {
		t.Fatalf("trailing IncrementToken error %v", err)
	}
	if ok {
		t.Errorf("trailing IncrementToken: got ok=true, want false")
	}
}

// TestCannedTokenStream_OffsetsAndTypeAndPayloadAndFlags exercises
// every non-default field of Token and the trailing
// finalPosInc/finalOffset applied by End().
func TestCannedTokenStream_OffsetsAndTypeAndPayloadAndFlags(t *testing.T) {
	t.Parallel()

	payload1 := []byte{0x01, 0x02}
	payload2 := []byte{0xff}
	tokens := []Token{
		NewToken("alpha", 0, 5).WithType("LETTER").WithFlags(0b0001).WithPayload(payload1),
		NewToken("beta", 6, 10).WithType("LETTER").WithFlags(0b0010).WithPayload(payload2),
		NewToken("gamma", 11, 16).WithType("").WithFlags(0), // explicit empty type honoured
	}
	cts := NewCannedTokenStreamWithFinal(2, 20, tokens...)

	type want struct {
		text    string
		start   int
		end     int
		typeStr string
		flags   int
		payload []byte
	}
	expectations := []want{
		{"alpha", 0, 5, "LETTER", 0b0001, payload1},
		{"beta", 6, 10, "LETTER", 0b0010, payload2},
		{"gamma", 11, 16, "", 0, nil},
	}

	for i, w := range expectations {
		ok, err := cts.IncrementToken()
		if err != nil || !ok {
			t.Fatalf("token #%d: IncrementToken ok=%v err=%v", i, ok, err)
		}
		if got := cts.CharTermAttribute().String(); got != w.text {
			t.Errorf("token #%d text: got %q, want %q", i, got, w.text)
		}
		if got := cts.TypeAttribute().GetType(); got != w.typeStr {
			t.Errorf("token #%d type: got %q, want %q", i, got, w.typeStr)
		}
		if got := cts.FlagsAttribute().GetFlags(); got != w.flags {
			t.Errorf("token #%d flags: got %d, want %d", i, got, w.flags)
		}
		got := cts.PayloadAttribute().GetPayload()
		if !bytes.Equal(got, w.payload) {
			t.Errorf("token #%d payload: got %v, want %v", i, got, w.payload)
		}
		// Payload must be defensively copied: mutating the source
		// must not affect the stream's attribute state.
		if i == 0 && len(payload1) > 0 {
			orig := payload1[0]
			payload1[0] = 0x7f
			if cts.PayloadAttribute().GetPayload()[0] == 0x7f {
				t.Errorf("token #%d: payload not copied defensively", i)
			}
			payload1[0] = orig
		}
	}

	if err := cts.End(); err != nil {
		t.Fatalf("End: unexpected error %v", err)
	}
	if got := cts.PositionIncrementAttribute().GetPositionIncrement(); got != 2 {
		t.Errorf("finalPosInc: got %d, want 2", got)
	}
	if start, end := cts.OffsetAttribute().StartOffset(), cts.OffsetAttribute().EndOffset(); start != 20 || end != 20 {
		t.Errorf("finalOffset: got (%d,%d), want (20,20)", start, end)
	}
}

// TestCannedTokenStream_Reset verifies that Reset rewinds the stream
// so the same canned sequence can be replayed.
func TestCannedTokenStream_Reset(t *testing.T) {
	t.Parallel()

	cts := NewCannedTokenStream(
		NewToken("one", 0, 3),
		NewToken("two", 4, 7),
	)

	consumeAll := func() []string {
		t.Helper()
		var out []string
		for {
			ok, err := cts.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken error %v", err)
			}
			if !ok {
				return out
			}
			out = append(out, cts.CharTermAttribute().String())
		}
	}

	first := consumeAll()
	if err := cts.Reset(); err != nil {
		t.Fatalf("Reset error %v", err)
	}
	second := consumeAll()
	if len(first) != len(second) {
		t.Fatalf("Reset: replay length mismatch: first=%v second=%v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("Reset: replay #%d mismatch: got %q, want %q", i, second[i], first[i])
		}
	}
}

// TestCannedTokenStream_AttributeSourceExposesAll verifies that every
// attribute is registered on the underlying AttributeSource, so
// downstream test harnesses (T4689 AssertTokenStreamContents) can
// look them up by type.
func TestCannedTokenStream_AttributeSourceExposesAll(t *testing.T) {
	t.Parallel()

	cts := NewCannedTokenStream(NewToken("x", 0, 1))
	src := cts.GetAttributeSource()

	if src.GetAttribute(analysis.CharTermAttributeType) == nil {
		t.Error("CharTermAttribute not registered")
	}
	if src.GetAttribute(analysis.OffsetAttributeType) == nil {
		t.Error("OffsetAttribute not registered")
	}
	if src.GetAttribute(analysis.TypeAttributeType) == nil {
		t.Error("TypeAttribute not registered")
	}
	if src.GetAttribute(analysis.PositionIncrementAttributeType) == nil {
		t.Error("PositionIncrementAttribute not registered")
	}
	if src.GetAttribute(analysis.PositionLengthAttributeType) == nil {
		t.Error("PositionLengthAttribute not registered")
	}
	if src.GetAttribute(analysis.FlagsAttributeType) == nil {
		t.Error("FlagsAttribute not registered")
	}
	if src.GetAttribute(analysis.PayloadAttributeType) == nil {
		t.Error("PayloadAttribute not registered")
	}
}
