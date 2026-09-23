// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// This file pins FloatPoint.setFloatValue(float), missing from the port: the
// call resolved to Field.SetFloatValue, which panics on the binary packed
// value ("cannot change value type"). Found by the faithful port of
// TestBaseRangeFilter.build, which reuses one FloatPoint across documents.

import (
	"bytes"
	"testing"
)

func TestFloatPoint_SetFloatValueRegression(t *testing.T) {
	fp := NewFloatPoint("f", 1)
	fp.SetFloatValue(7.5)
	if want := PackFloatsLucene(7.5); !bytes.Equal(fp.PointValues(), want) {
		t.Fatalf("packed = %v, want %v", fp.PointValues(), want)
	}
	if got := fp.FloatValue(); got != 7.5 {
		t.Fatalf("FloatValue() = %v, want 7.5", got)
	}
}
