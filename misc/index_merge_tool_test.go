// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.misc.TestIndexMergeTool.
package misc_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/misc"
)

// TestIndexMergeTool_NoParameters mirrors testNoParameters.
// Passing zero arguments must return an error.
func TestIndexMergeTool_NoParameters(t *testing.T) {
	_, err := misc.ParseIndexMergeOptions([]string{})
	if err == nil {
		t.Fatal("expected error for zero arguments, got nil")
	}
}

// TestIndexMergeTool_OneParameter mirrors testOneParameter.
// Passing only a target path (no sources) must return an error.
func TestIndexMergeTool_OneParameter(t *testing.T) {
	_, err := misc.ParseIndexMergeOptions([]string{"target"})
	if err == nil {
		t.Fatal("expected error for one argument, got nil")
	}
}

// TestIndexMergeTool_TwoParameters mirrors testTwoParameters.
// Passing target and one source must return an error (need at least two sources).
func TestIndexMergeTool_TwoParameters(t *testing.T) {
	_, err := misc.ParseIndexMergeOptions([]string{"target", "source1"})
	if err == nil {
		t.Fatal("expected error for two arguments, got nil")
	}
}

// TestIndexMergeTool_ThreeParameters mirrors testThreeParameters.
// Passing target and two sources must succeed and populate the options.
func TestIndexMergeTool_ThreeParameters(t *testing.T) {
	opts, err := misc.ParseIndexMergeOptions([]string{"target", "source1", "source2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.MergedIndexPath != "target" {
		t.Errorf("MergedIndexPath: expected %q, got %q", "target", opts.MergedIndexPath)
	}
	want := []string{"source1", "source2"}
	if len(opts.IndexPaths) != len(want) {
		t.Fatalf("IndexPaths: expected %v, got %v", want, opts.IndexPaths)
	}
	for i, w := range want {
		if opts.IndexPaths[i] != w {
			t.Errorf("IndexPaths[%d]: expected %q, got %q", i, w, opts.IndexPaths[i])
		}
	}
}

// TestIndexMergeTool_VerboseOption mirrors testVerboseOption.
// The -verbose flag must set Verbose = true.
func TestIndexMergeTool_VerboseOption(t *testing.T) {
	opts, err := misc.ParseIndexMergeOptions([]string{"-verbose", "target", "source1", "source2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Verbose {
		t.Error("expected Verbose=true after -verbose flag")
	}
}

// TestIndexMergeTool_MergePolicyOption mirrors testMergePolicyOption.
// The -merge-policy flag must capture the class name.
func TestIndexMergeTool_MergePolicyOption(t *testing.T) {
	className := "org.apache.lucene.index.LogDocMergePolicy"
	opts, err := misc.ParseIndexMergeOptions([]string{
		"-merge-policy", className, "target", "source1", "source2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.MergePolicyClassName != className {
		t.Errorf("MergePolicyClassName: expected %q, got %q", className, opts.MergePolicyClassName)
	}
}

// TestIndexMergeTool_MaxSegmentsOption mirrors testMaxSegmentsOption.
// The -max-segments flag must be parsed as an integer.
func TestIndexMergeTool_MaxSegmentsOption(t *testing.T) {
	opts, err := misc.ParseIndexMergeOptions([]string{
		"-max-segments", "42", "target", "source1", "source2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.MaxSegments != 42 {
		t.Errorf("MaxSegments: expected 42, got %d", opts.MaxSegments)
	}
}

// TestIndexMergeTool_UnrecognizedOption verifies that an unknown flag returns an error.
func TestIndexMergeTool_UnrecognizedOption(t *testing.T) {
	_, err := misc.ParseIndexMergeOptions([]string{
		"-unknown-flag", "target", "source1", "source2",
	})
	if err == nil {
		t.Fatal("expected error for unrecognized option, got nil")
	}
}
