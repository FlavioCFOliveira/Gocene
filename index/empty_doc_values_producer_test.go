// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"
)

func TestEmptyDocValuesProducer_AllAccessorsReturnError(t *testing.T) {
	p := NewEmptyDocValuesProducer()
	if _, err := p.GetNumeric(nil); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("GetNumeric: %v", err)
	}
	if _, err := p.GetBinary(nil); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("GetBinary: %v", err)
	}
	if _, err := p.GetSorted(nil); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("GetSorted: %v", err)
	}
	if _, err := p.GetSortedNumeric(nil); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("GetSortedNumeric: %v", err)
	}
	if _, err := p.GetSortedSet(nil); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("GetSortedSet: %v", err)
	}
	if _, err := p.GetSkipper(nil); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("GetSkipper: %v", err)
	}
	if err := p.CheckIntegrity(); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("CheckIntegrity: %v", err)
	}
	if err := p.Close(); !errors.Is(err, errUnsupportedEmptyDV) {
		t.Errorf("Close: %v", err)
	}
}
