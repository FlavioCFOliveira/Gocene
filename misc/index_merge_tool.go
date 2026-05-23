// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package misc provides miscellaneous utilities for Gocene.
// This file ports org.apache.lucene.misc.IndexMergeTool.
package misc

import (
	"fmt"
	"strconv"
)

// indexMergeToolUsage mirrors the USAGE constant in the Java source.
const indexMergeToolUsage = "Usage: IndexMergeTool [OPTION...] <mergedIndex> <index1> <index2> [index3] ...\n" +
	"Merges source indexes 'index1' .. 'indexN' into 'mergedIndex'\n" +
	"\n" +
	"OPTIONS:\n" +
	" -merge-policy ClassName  specifies MergePolicy class. The default is TieredMergePolicy.\n" +
	" -max-segments N          force-merge's the index to a maximum of N segments. Default is\n" +
	"                          to execute only the merges according to the merge policy.\n" +
	" -verbose                 print additional details.\n"

// IndexMergeOptions holds the parsed command-line options for IndexMergeTool.
// It mirrors org.apache.lucene.misc.IndexMergeTool.Options.
type IndexMergeOptions struct {
	// MergedIndexPath is the destination index path (first positional argument).
	MergedIndexPath string

	// IndexPaths holds the source index paths (second and subsequent positional arguments).
	IndexPaths []string

	// MergePolicyClassName is the optional fully-qualified merge-policy class name.
	// Empty string means default (TieredMergePolicy).
	MergePolicyClassName string

	// MaxSegments is the optional force-merge target (0 means no force-merge).
	MaxSegments int

	// Verbose requests additional diagnostic output.
	Verbose bool
}

// ParseIndexMergeOptions parses the given argument slice into IndexMergeOptions.
// It returns a non-nil error (wrapping the usage string) when validation fails,
// matching the IllegalArgumentException semantics of the Java original.
func ParseIndexMergeOptions(args []string) (*IndexMergeOptions, error) {
	opts := &IndexMergeOptions{}
	idx := 0

	for idx < len(args) {
		arg := args[idx]
		if !startsWith(arg, "-") {
			break
		}
		if arg == "--" {
			idx++
			break
		}
		switch arg {
		case "-merge-policy":
			idx++
			if idx >= len(args) {
				return nil, fmt.Errorf("missing value for -merge-policy\n%s", indexMergeToolUsage)
			}
			opts.MergePolicyClassName = args[idx]
		case "-max-segments":
			idx++
			if idx >= len(args) {
				return nil, fmt.Errorf("missing value for -max-segments\n%s", indexMergeToolUsage)
			}
			n, err := strconv.Atoi(args[idx])
			if err != nil {
				return nil, fmt.Errorf("invalid value for -max-segments: %q\n%s", args[idx], indexMergeToolUsage)
			}
			opts.MaxSegments = n
		case "-verbose":
			opts.Verbose = true
		default:
			return nil, fmt.Errorf("unrecognized option: %q\n%s", arg, indexMergeToolUsage)
		}
		idx++
	}

	numPaths := len(args) - idx
	if numPaths < 3 {
		return nil, fmt.Errorf("not enough parameters.\n%s", indexMergeToolUsage)
	}

	opts.MergedIndexPath = args[idx]
	opts.IndexPaths = make([]string, numPaths-1)
	copy(opts.IndexPaths, args[idx+1:])
	return opts, nil
}

// startsWith returns true when s begins with prefix.
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// IndexMergeTool merges indices specified on the command line into the index
// specified as the first argument. It is a port of
// org.apache.lucene.misc.IndexMergeTool. The actual I/O merge step is
// injected via MergeFunc so the tool can be embedded in larger programs
// without spawning a subprocess.
type IndexMergeTool struct {
	opts *IndexMergeOptions
}

// NewIndexMergeTool constructs an IndexMergeTool from pre-parsed options.
func NewIndexMergeTool(opts *IndexMergeOptions) *IndexMergeTool {
	return &IndexMergeTool{opts: opts}
}

// NewIndexMergeToolFromArgs parses args and constructs an IndexMergeTool.
// Returns an error if argument parsing fails.
func NewIndexMergeToolFromArgs(args []string) (*IndexMergeTool, error) {
	opts, err := ParseIndexMergeOptions(args)
	if err != nil {
		return nil, err
	}
	return &IndexMergeTool{opts: opts}, nil
}

// Options returns the parsed options this tool was constructed with.
func (t *IndexMergeTool) Options() *IndexMergeOptions { return t.opts }

// Run executes the merge by invoking merge. The merge function receives the
// destination path, source paths, and the options so it can respect
// MaxSegments and Verbose.
func (t *IndexMergeTool) Run(merge func(opts *IndexMergeOptions) error) error {
	if merge == nil {
		return nil
	}
	return merge(t.opts)
}
