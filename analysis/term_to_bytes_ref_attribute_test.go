// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubTermToBytesRefAttribute is a minimal in-test impl exercised in
// place of a Lucene Java test peer; the Lucene reference
// (TermToBytesRefAttribute.java) ships no direct TestTermToBytesRefAttribute.java,
// so this test asserts the interface contract documented in the
// reference Javadoc.
type stubTermToBytesRefAttribute struct {
	bytes *util.BytesRef
}

func (s *stubTermToBytesRefAttribute) GetBytesRef() *util.BytesRef {
	return s.bytes
}

// TestTermToBytesRefAttribute_InterfaceContract verifies that the
// interface is satisfied by an impl whose GetBytesRef returns the
// current BytesRef and that values returned on successive calls reflect
// mutations of the underlying term (matching the Lucene contract that
// the returned BytesRef may be reused).
func TestTermToBytesRefAttribute_InterfaceContract(t *testing.T) {
	var attr TermToBytesRefAttribute = &stubTermToBytesRefAttribute{
		bytes: util.NewBytesRef([]byte("hello")),
	}

	got := attr.GetBytesRef()
	if got == nil || got.String() != "hello" {
		t.Fatalf("GetBytesRef returned %#v, want BytesRef(hello)", got)
	}

	// Mutate the underlying ref to model the "reused on increment" path.
	stub := attr.(*stubTermToBytesRefAttribute)
	stub.bytes = util.NewBytesRef([]byte("world"))
	if got := attr.GetBytesRef(); got == nil || got.String() != "world" {
		t.Fatalf("GetBytesRef after mutation: %#v, want BytesRef(world)", got)
	}
}
