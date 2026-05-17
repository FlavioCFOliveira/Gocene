// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"
)

func TestCorruptIndexException_Message(t *testing.T) {
	e := NewCorruptIndexException("bad header", "_0.fnm")
	want := "bad header (resource=_0.fnm)"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
	if e.GetOriginalMessage() != "bad header" {
		t.Errorf("GetOriginalMessage() = %q", e.GetOriginalMessage())
	}
	if e.GetResourceDescription() != "_0.fnm" {
		t.Errorf("GetResourceDescription() = %q", e.GetResourceDescription())
	}
}

func TestCorruptIndexException_WithCause(t *testing.T) {
	root := errors.New("eof")
	e := NewCorruptIndexExceptionWithCause("checksum mismatch", "_1.cfs", root)
	if !errors.Is(e, root) {
		t.Errorf("errors.Is(e, root) = false")
	}
	want := "checksum mismatch (resource=_1.cfs): eof"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
}

func TestIndexFormatTooNewException_Message(t *testing.T) {
	e := NewIndexFormatTooNewException("seg.si", 9, 0, 7)
	want := "Format version is not supported (resource seg.si): 9 (needs to be between 0 and 7)"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
	if e.GetVersion() != 9 || e.GetMinVersion() != 0 || e.GetMaxVersion() != 7 {
		t.Errorf("version accessors mismatch: v=%d min=%d max=%d", e.GetVersion(), e.GetMinVersion(), e.GetMaxVersion())
	}
	if e.GetResourceDescription() != "seg.si" {
		t.Errorf("GetResourceDescription() = %q", e.GetResourceDescription())
	}
}

func TestIndexFormatTooOldException_VersionForm(t *testing.T) {
	e := NewIndexFormatTooOldException("seg.si", 3, 7, 9)
	got := e.Error()
	// Allow MinSupportedMajor to vary with util.LuceneVersionMajor.
	want := "Format version is not supported (resource seg.si): 3 (needs to be between 7 and 9). This version of Lucene only supports indexes created with release"
	if len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("Error() prefix mismatch:\n got %q\nwant prefix %q", got, want)
	}
	if e.GetVersion() == nil || *e.GetVersion() != 3 {
		t.Errorf("GetVersion() = %v", e.GetVersion())
	}
	if e.GetReason() != "" {
		t.Errorf("GetReason() = %q, want empty", e.GetReason())
	}
}

func TestIndexFormatTooOldException_ReasonForm(t *testing.T) {
	e := NewIndexFormatTooOldExceptionReason("seg.si", "unsupported magic")
	got := e.Error()
	want := "Format version is not supported (resource seg.si): unsupported magic. This version of Lucene only supports indexes created with release"
	if len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("Error() prefix mismatch:\n got %q\nwant prefix %q", got, want)
	}
	if e.GetVersion() != nil {
		t.Errorf("GetVersion() = %v, want nil", e.GetVersion())
	}
	if e.GetReason() != "unsupported magic" {
		t.Errorf("GetReason() = %q", e.GetReason())
	}
}

func TestIndexNotFoundException_FromMessage(t *testing.T) {
	e := IndexNotFoundExceptionFromMessage("no segments file")
	if e.Error() != "no segments file" {
		t.Errorf("Error() = %q", e.Error())
	}
	if e.Cause != nil {
		t.Errorf("Cause should be nil")
	}
}
