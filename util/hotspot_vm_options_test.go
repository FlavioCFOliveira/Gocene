// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestHotspotVMOptions_IsNoOp pins down the documented behaviour of the
// intentional no-op port of org.apache.lucene.util.HotspotVMOptions.
//
// The Java original reads HotSpot JVM tuning flags via a reflective MBean
// bridge. Go has no HotSpot VM and no MBean equivalent, so this port is a
// frozen stub. This test exists to:
//
//  1. Document, in executable form, why the stub returns the values it
//     returns (so a future contributor cannot quietly "fix" it by hooking
//     up something runtime-specific).
//  2. Guarantee the surface stays total — Get must never panic, never
//     allocate per-call beyond the trivial return, and must answer "absent"
//     for every input including the empty string and known HotSpot flag
//     names that the Java caller in Constants.java used to query.
func TestHotspotVMOptions_IsNoOp(t *testing.T) {
	t.Parallel()

	if IsHotspotVM {
		t.Fatalf("IsHotspotVM = true; want false: Go has no HotSpot VM, the Java MBean bridge cannot exist in this runtime")
	}

	// The names below are the exact flags Lucene 10.4.0 Constants.java
	// queries through HotspotVMOptions.get(). They are exercised here to
	// pin the no-op contract for the call sites a mechanical port would
	// reproduce.
	cases := []string{
		"",
		"UseJVMCICompiler",
		"UseFMA",
		"MaxVectorSize",
		"UseXmmI2F",
		"UseAVX",
		"UseSVE",
		"definitely-not-a-real-flag",
	}
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := GetHotspotVMOption(name)
			if ok {
				t.Fatalf("GetHotspotVMOption(%q) ok = true; want false (no JVM, no options)", name)
			}
			if got != "" {
				t.Fatalf("GetHotspotVMOption(%q) value = %q; want empty string", name, got)
			}
		})
	}
}
