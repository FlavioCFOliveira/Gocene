// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"
)

// TestLucene80NormsConsumer_Name verifies that the NormsFormat name matches.
func TestLucene80NormsConsumer_Name(t *testing.T) {
	f := NewLucene80NormsFormat()
	if got := f.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}
