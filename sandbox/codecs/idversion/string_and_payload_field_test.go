// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.codecs.idversion.StringAndPayloadField.
//
// StringAndPayloadField is a test-support type used by
// TestIDVersionPostingsFormat. It produces a single token carrying a payload.
// In Go it is placed in the idversion package's test file rather than as a
// separate production file (the Java source lives under src/test/...).
package idversion

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// singleTokenWithPayloadTokenStream mirrors
// StringAndPayloadField.SingleTokenWithPayloadTokenStream.
//
// It emits exactly one token: the configured string term with the configured
// payload. Subsequent IncrementToken calls return false until Reset is called.
type singleTokenWithPayloadTokenStream struct {
	*analysis.BaseTokenStream
	termAttr    analysis.CharTermAttribute
	payloadAttr analysis.PayloadAttribute
	used        bool
	value       string
	payload     []byte
}

// setValue sets the term and payload for the next token.
func (ts *singleTokenWithPayloadTokenStream) setValue(value string, payload []byte) {
	ts.value = value
	ts.payload = payload
}

// IncrementToken emits the configured token exactly once, then returns false.
func (ts *singleTokenWithPayloadTokenStream) IncrementToken() (bool, error) {
	if ts.used {
		return false, nil
	}
	ts.ClearAttributes()
	ts.termAttr.AppendString(ts.value)
	ts.payloadAttr.SetPayload(ts.payload)
	ts.used = true
	return true, nil
}

// Reset makes the stream reusable.
func (ts *singleTokenWithPayloadTokenStream) Reset() error {
	ts.used = false
	return nil
}

// Close clears references.
func (ts *singleTokenWithPayloadTokenStream) Close() error {
	ts.value = ""
	ts.payload = nil
	return ts.BaseTokenStream.Close()
}

// ---- tests ----

// TestSingleTokenWithPayloadTokenStream_EmitsOneToken verifies that
// IncrementToken returns true exactly once and false on subsequent calls.
func TestSingleTokenWithPayloadTokenStream_EmitsOneToken(t *testing.T) {
	ts := &singleTokenWithPayloadTokenStream{
		BaseTokenStream: analysis.NewBaseTokenStream(),
	}
	ts.termAttr = analysis.NewCharTermAttribute()
	ts.payloadAttr = analysis.NewPayloadAttribute()
	ts.setValue("hello", []byte{1, 2, 3})

	ok, err := ts.IncrementToken()
	if err != nil || !ok {
		t.Fatalf("first IncrementToken: ok=%v err=%v; want true,nil", ok, err)
	}
	ok, err = ts.IncrementToken()
	if err != nil || ok {
		t.Fatalf("second IncrementToken: ok=%v err=%v; want false,nil", ok, err)
	}
}

// TestSingleTokenWithPayloadTokenStream_ResetAllowsReuse verifies that Reset
// allows the stream to emit the token again.
func TestSingleTokenWithPayloadTokenStream_ResetAllowsReuse(t *testing.T) {
	ts := &singleTokenWithPayloadTokenStream{
		BaseTokenStream: analysis.NewBaseTokenStream(),
	}
	ts.termAttr = analysis.NewCharTermAttribute()
	ts.payloadAttr = analysis.NewPayloadAttribute()
	ts.setValue("world", []byte{42})

	if ok, _ := ts.IncrementToken(); !ok {
		t.Fatal("first IncrementToken should return true")
	}
	if ok, _ := ts.IncrementToken(); ok {
		t.Fatal("second IncrementToken should return false")
	}
	if err := ts.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if ok, _ := ts.IncrementToken(); !ok {
		t.Fatal("after Reset, IncrementToken should return true again")
	}
}

// TestSingleTokenWithPayloadTokenStream_SetValueChangesToken verifies that
// setValue updates both the term and the payload for the next emission.
func TestSingleTokenWithPayloadTokenStream_SetValueChangesToken(t *testing.T) {
	ts := &singleTokenWithPayloadTokenStream{
		BaseTokenStream: analysis.NewBaseTokenStream(),
	}
	ts.termAttr = analysis.NewCharTermAttribute()
	ts.payloadAttr = analysis.NewPayloadAttribute()
	ts.setValue("first", []byte{1})

	if ok, _ := ts.IncrementToken(); !ok {
		t.Fatal("expected first token")
	}
	// Payload should be the one set by setValue.
	if got := ts.payloadAttr.GetPayload(); len(got) != 1 || got[0] != 1 {
		t.Errorf("payload = %v; want [1]", got)
	}

	// Change and reuse.
	ts.Reset()
	ts.setValue("second", []byte{99, 100})
	if ok, _ := ts.IncrementToken(); !ok {
		t.Fatal("expected second token after setValue+Reset")
	}
	if got := ts.payloadAttr.GetPayload(); len(got) != 2 || got[0] != 99 || got[1] != 100 {
		t.Errorf("payload = %v; want [99 100]", got)
	}
}
