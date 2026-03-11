// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

func TestIOContext(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "default contexts",
			fn: func(t *testing.T) {
				if IOContextRead.Context != ContextRead {
					t.Error("IOContextRead should have ContextRead")
				}
				if IOContextWrite.Context != ContextWrite {
					t.Error("IOContextWrite should have ContextWrite")
				}
				if IOContextReadOnce.Context != ContextReadOnce {
					t.Error("IOContextReadOnce should have ContextReadOnce")
				}
				if !IOContextReadOnce.ReadOnce {
					t.Error("IOContextReadOnce should have ReadOnce=true")
				}
			},
		},
		{
			name: "io context type string",
			fn: func(t *testing.T) {
				tests := []struct {
					ctx      IOContextType
					expected string
				}{
					{ContextRead, "READ"},
					{ContextWrite, "WRITE"},
					{ContextMerge, "MERGE"},
					{ContextFlush, "FLUSH"},
					{ContextReadOnce, "READONCE"},
					{IOContextType(99), "UNKNOWN"},
				}

				for _, tt := range tests {
					if got := tt.ctx.String(); got != tt.expected {
						t.Errorf("%v.String() = %q, want %q", tt.ctx, got, tt.expected)
					}
				}
			},
		},
		{
			name: "new merge context",
			fn: func(t *testing.T) {
				mergeInfo := &MergeInfo{
					TotalMaxDoc:         1000,
					EstimatedMergeBytes: 1024 * 1024,
					IsExternal:          false,
					MergeFactor:         10,
				}

				ctx := NewMergeContext(mergeInfo)

				if ctx.Context != ContextMerge {
					t.Error("expected ContextMerge")
				}
				if ctx.MergeInfo != mergeInfo {
					t.Error("expected MergeInfo to be set")
				}
				if !ctx.IsMerge() {
					t.Error("expected IsMerge() to be true")
				}
			},
		},
		{
			name: "new flush context",
			fn: func(t *testing.T) {
				flushInfo := &FlushInfo{
					NumDocs:              100,
					EstimatedSegmentSize: 1024,
				}

				ctx := NewFlushContext(flushInfo)

				if ctx.Context != ContextFlush {
					t.Error("expected ContextFlush")
				}
				if ctx.FlushInfo != flushInfo {
					t.Error("expected FlushInfo to be set")
				}
				if !ctx.IsFlush() {
					t.Error("expected IsFlush() to be true")
				}
			},
		},
		{
			name: "is read",
			fn: func(t *testing.T) {
				if !IOContextRead.IsRead() {
					t.Error("IOContextRead should be read")
				}
				if !IOContextReadOnce.IsRead() {
					t.Error("IOContextReadOnce should be read")
				}
				if IOContextWrite.IsRead() {
					t.Error("IOContextWrite should not be read")
				}
			},
		},
		{
			name: "is write",
			fn: func(t *testing.T) {
				if !IOContextWrite.IsWrite() {
					t.Error("IOContextWrite should be write")
				}
				if IOContextRead.IsWrite() {
					t.Error("IOContextRead should not be write")
				}
			},
		},
		{
			name: "merge info fields",
			fn: func(t *testing.T) {
				info := &MergeInfo{
					TotalMaxDoc:         5000,
					EstimatedMergeBytes: 5000000,
					IsExternal:          true,
					MergeFactor:         5,
				}

				if info.TotalMaxDoc != 5000 {
					t.Error("TotalMaxDoc mismatch")
				}
				if info.EstimatedMergeBytes != 5000000 {
					t.Error("EstimatedMergeBytes mismatch")
				}
				if !info.IsExternal {
					t.Error("IsExternal should be true")
				}
				if info.MergeFactor != 5 {
					t.Error("MergeFactor mismatch")
				}
			},
		},
		{
			name: "flush info fields",
			fn: func(t *testing.T) {
				info := &FlushInfo{
					NumDocs:              200,
					EstimatedSegmentSize: 2048,
				}

				if info.NumDocs != 200 {
					t.Error("NumDocs mismatch")
				}
				if info.EstimatedSegmentSize != 2048 {
					t.Error("EstimatedSegmentSize mismatch")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestLock(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "native fs lock factory",
			fn: func(t *testing.T) {
				factory := NewNativeFSLockFactory()
				if factory == nil {
					t.Fatal("expected non-nil factory")
				}

				// Create a mock directory for obtaining lock
				dir := NewBaseDirectory(nil)
				lock, err := factory.ObtainLock(dir, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if lock == nil {
					t.Fatal("expected non-nil lock")
				}

				if !lock.IsLocked() {
					t.Error("expected lock to be locked")
				}

				if err := lock.Close(); err != nil {
					t.Errorf("unexpected error closing lock: %v", err)
				}

				if lock.IsLocked() {
					t.Error("expected lock to be unlocked after close")
				}
			},
		},
		{
			name: "native fs lock ensure valid",
			fn: func(t *testing.T) {
				factory := NewNativeFSLockFactory()
				dir := NewBaseDirectory(nil)
				lock, err := factory.ObtainLock(dir, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if err := lock.EnsureValid(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				lock.Close()

				if err := lock.EnsureValid(); err == nil {
					t.Error("expected error after closing lock")
				}
			},
		},
		{
			name: "single instance lock factory",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()
				if factory == nil {
					t.Fatal("expected non-nil factory")
				}

				dir := NewBaseDirectory(nil)
				lock1, err := factory.ObtainLock(dir, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				// Should fail to obtain same lock twice
				_, err = factory.ObtainLock(dir, "test.lock")
				if err == nil {
					t.Error("expected error when obtaining same lock twice")
				}

				// Close first lock
				if err := lock1.Close(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Now should succeed
				lock2, err := factory.ObtainLock(dir, "test.lock")
				if err != nil {
					t.Errorf("unexpected error after closing: %v", err)
				}
				if lock2 == nil {
					t.Error("expected non-nil lock")
				}
			},
		},
		{
			name: "single instance lock ensure valid",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()
				dir := NewBaseDirectory(nil)
				lock, _ := factory.ObtainLock(dir, "test.lock")

				if err := lock.EnsureValid(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				lock.Close()

				if err := lock.EnsureValid(); err == nil {
					t.Error("expected error after close")
				}
			},
		},
		{
			name: "no lock factory",
			fn: func(t *testing.T) {
				factory := NewNoLockFactory()
				if factory == nil {
					t.Fatal("expected non-nil factory")
				}

				dir := NewBaseDirectory(nil)
				lock, err := factory.ObtainLock(dir, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				// No-op lock is never locked
				if lock.IsLocked() {
					t.Error("expected no-op lock to not be locked")
				}

				// Close does nothing
				if err := lock.Close(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// EnsureValid does nothing
				if err := lock.EnsureValid(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "base lock",
			fn: func(t *testing.T) {
				bl := NewBaseLock()
				if bl == nil {
					t.Fatal("expected non-nil base lock")
				}

				if !bl.IsLocked() {
					t.Error("expected lock to be locked initially")
				}

				if err := bl.VerifyLocked(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				bl.MarkReleased()

				if bl.IsLocked() {
					t.Error("expected lock to be released")
				}

				if err := bl.VerifyLocked(); err == nil {
					t.Error("expected error after release")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}
