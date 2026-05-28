// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package payloads

import (
	"bytes"
	"testing"
)

// TestPayloadSpanCollectorCollectLeaf verifies that collected payloads are
// stored in order and, critically, that CollectLeaf copies the input so a
// later mutation of the caller's buffer cannot corrupt an already-collected
// payload.
func TestPayloadSpanCollectorCollectLeaf(t *testing.T) {
	t.Parallel()
	c := NewPayloadSpanCollector()
	if c == nil {
		t.Fatal("NewPayloadSpanCollector() = nil")
	}
	if len(c.Payloads) != 0 {
		t.Fatalf("fresh collector has %d payloads, want 0", len(c.Payloads))
	}

	first := []byte{1, 2, 3}
	c.CollectLeaf(first)
	c.CollectLeaf([]byte{4, 5})

	if len(c.Payloads) != 2 {
		t.Fatalf("after two CollectLeaf calls, len = %d, want 2", len(c.Payloads))
	}
	if !bytes.Equal(c.Payloads[0], []byte{1, 2, 3}) {
		t.Errorf("Payloads[0] = % x, want 01 02 03", c.Payloads[0])
	}
	if !bytes.Equal(c.Payloads[1], []byte{4, 5}) {
		t.Errorf("Payloads[1] = % x, want 04 05", c.Payloads[1])
	}

	// Mutating the caller's original slice must not affect the stored copy.
	first[0] = 0xFF
	if !bytes.Equal(c.Payloads[0], []byte{1, 2, 3}) {
		t.Errorf("CollectLeaf did not copy: Payloads[0] = % x after source mutation, want 01 02 03", c.Payloads[0])
	}
}

// TestPayloadSpanCollectorReset verifies Reset empties the collected payloads.
func TestPayloadSpanCollectorReset(t *testing.T) {
	t.Parallel()
	c := NewPayloadSpanCollector()
	c.CollectLeaf([]byte{9})
	c.CollectLeaf([]byte{8})
	if len(c.Payloads) != 2 {
		t.Fatalf("len before Reset = %d, want 2", len(c.Payloads))
	}

	c.Reset()
	if len(c.Payloads) != 0 {
		t.Errorf("len after Reset = %d, want 0", len(c.Payloads))
	}

	// Collection still works after Reset.
	c.CollectLeaf([]byte{7})
	if len(c.Payloads) != 1 || !bytes.Equal(c.Payloads[0], []byte{7}) {
		t.Errorf("after Reset+CollectLeaf, Payloads = %v, want [[7]]", c.Payloads)
	}
}

// TestPayloadSpanUtilGetPayloads verifies the util flattens a collector's
// payloads into a fresh slice and guards against a nil collector.
func TestPayloadSpanUtilGetPayloads(t *testing.T) {
	t.Parallel()
	var util PayloadSpanUtil

	if got := util.GetPayloads(nil); got != nil {
		t.Errorf("GetPayloads(nil) = %v, want nil", got)
	}

	c := NewPayloadSpanCollector()
	c.CollectLeaf([]byte{1})
	c.CollectLeaf([]byte{2})

	got := util.GetPayloads(c)
	if len(got) != 2 {
		t.Fatalf("GetPayloads len = %d, want 2", len(got))
	}
	if !bytes.Equal(got[0], []byte{1}) || !bytes.Equal(got[1], []byte{2}) {
		t.Errorf("GetPayloads = %v, want [[1] [2]]", got)
	}
}
