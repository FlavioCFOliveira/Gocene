// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "testing"

func TestInvertableType_String(t *testing.T) {
	cases := map[InvertableType]string{
		InvertableTypeBinary:      "BINARY",
		InvertableTypeTokenStream: "TOKEN_STREAM",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Fatalf("InvertableType(%d).String() = %q, want %q", int(v), got, want)
		}
	}
	if InvertableType(99).String() != "UNKNOWN(99)" {
		t.Fatalf("unexpected fallback name")
	}
}

func TestInvertableType_Ordinal(t *testing.T) {
	if InvertableTypeBinary.Ordinal() != 0 {
		t.Fatalf("Binary ordinal must be 0")
	}
	if InvertableTypeTokenStream.Ordinal() != 1 {
		t.Fatalf("TokenStream ordinal must be 1")
	}
}
