// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"testing"
)

// TestDoubleMetaphone_Basic validates the DoubleMetaphone encoder against the
// expected outputs from TestDoubleMetaphoneFilter.java and TestPhoneticFilter.java.
// Source: analysis/phonetic/src/test/.../TestDoubleMetaphoneFilter.java
func TestDoubleMetaphone_Basic(t *testing.T) {
	enc := NewDoubleMetaphone()

	tests := []struct {
		input   string
		maxLen  int
		primary string
		alt     string
	}{
		// From TestDoubleMetaphoneFilter (inject=false, maxCodeLength=4)
		{"international", 4, "ANTR", ""},
		// From TestDoubleMetaphoneFilter (inject=false, maxCodeLength=8)
		{"international", 8, "ANTRNXNL", ""},
		// Kuczewski → KSSK (primary), KXFS (alternate)
		{"Kuczewski", 4, "KSSK", "KXFS"},
		// From TestPhoneticFilter
		{"aaa", 4, "A", ""},
		{"bbb", 4, "PP", ""},
		{"ccc", 4, "KK", "KK"},
		{"easgasg", 4, "ASKS", ""},
	}

	for _, tt := range tests {
		enc.MaxCodeLen = tt.maxLen
		p, a := enc.DoubleMetaphoneValue(tt.input)
		if tt.primary != "" && p != tt.primary {
			t.Errorf("Primary(%q, maxLen=%d) = %q, want %q", tt.input, tt.maxLen, p, tt.primary)
		}
		if tt.alt != "" && a != tt.alt {
			t.Errorf("Alternate(%q, maxLen=%d) = %q, want %q", tt.input, tt.maxLen, a, tt.alt)
		}
	}
}
