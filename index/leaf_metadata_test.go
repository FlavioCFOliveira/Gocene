// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestLeafMetaData_VersionInvariants(t *testing.T) {
	// createdVersionMajor too high -> error
	if _, err := NewLeafMetaData(util.LuceneVersionMajor+1, nil, nil, false); err == nil {
		t.Errorf("expected error for version > LATEST")
	}
	// createdVersionMajor < 6 -> error
	if _, err := NewLeafMetaData(5, nil, nil, false); err == nil {
		t.Errorf("expected error for version < 6")
	}
	// createdVersionMajor >= 7 requires minVersion
	if _, err := NewLeafMetaData(7, nil, nil, false); err == nil {
		t.Errorf("expected error for >=7 without minVersion")
	}
	// version 6 with no minVersion is allowed
	if _, err := NewLeafMetaData(6, nil, nil, false); err != nil {
		t.Errorf("expected ok for v=6, got %v", err)
	}
	// version 9 with minVersion works
	v := util.NewVersion(9, 0, 0)
	md, err := NewLeafMetaData(9, v, nil, true)
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}
	if md.CreatedVersionMajor() != 9 {
		t.Errorf("CreatedVersionMajor = %d", md.CreatedVersionMajor())
	}
	if md.MinVersion() == nil || md.MinVersion().Major != 9 {
		t.Errorf("MinVersion mismatch: %v", md.MinVersion())
	}
	if !md.HasBlocks() {
		t.Errorf("HasBlocks=false")
	}
	if md.Sort() != nil {
		t.Errorf("Sort expected nil")
	}
}

type stubIndexSort struct{ name string }

func (s stubIndexSort) String() string { return s.name }

func TestLeafMetaData_Sort(t *testing.T) {
	v := util.NewVersion(10, 0, 0)
	md, err := NewLeafMetaData(10, v, stubIndexSort{name: "by_id"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if md.Sort() == nil || md.Sort().String() != "by_id" {
		t.Errorf("Sort accessor mismatch")
	}
}
