// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene84

import "testing"

func TestForDeltaUtil_Name(t *testing.T) {
	u := NewForDeltaUtil("1.0")
	if u.Name != "ForDeltaUtil" {
		t.Fatalf("got Name=%q, want %q", u.Name, "ForDeltaUtil")
	}
}

func TestPForUtil_Name(t *testing.T) {
	u := NewPForUtil("1.0")
	if u.Name != "PForUtil" {
		t.Fatalf("got Name=%q, want %q", u.Name, "PForUtil")
	}
}

func TestForDeltaUtil_Version(t *testing.T) {
	u := NewForDeltaUtil("2.0")
	if u.Version != "2.0" {
		t.Fatalf("got Version=%q, want %q", u.Version, "2.0")
	}
}

func TestPForUtil_Version(t *testing.T) {
	u := NewPForUtil("3.0")
	if u.Version != "3.0" {
		t.Fatalf("got Version=%q, want %q", u.Version, "3.0")
	}
}

func TestForDeltaUtil_NewReturnsNonNil(t *testing.T) {
	u := NewForDeltaUtil("1.0")
	if u == nil {
		t.Fatal("NewForDeltaUtil returned nil")
	}
}

func TestPForUtil_NewReturnsNonNil(t *testing.T) {
	u := NewPForUtil("1.0")
	if u == nil {
		t.Fatal("NewPForUtil returned nil")
	}
}
