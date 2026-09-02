// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

func TestLucene80DocValuesFormat(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	if f.Name() != "Lucene80" {
		t.Errorf("expected Lucene80, got %s", f.Name())
	}
}

func TestLucene80NormsFormat(t *testing.T) {
	f := NewLucene80NormsFormat()
	if f.Name() != "Lucene80Norms" {
		t.Errorf("expected Lucene80Norms, got %s", f.Name())
	}
}
