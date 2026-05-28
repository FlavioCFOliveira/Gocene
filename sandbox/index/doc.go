// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index holds experimental index-layer extensions ported from
// org.apache.lucene.sandbox.index.
//
// The sandbox modules collect components that are useful but not yet
// considered stable enough for the core index/ package; their APIs may
// change or be promoted without the usual back-compatibility guarantees.
//
// Currently this package provides MergeOnFlushMergePolicy, a MergePolicy
// wrapper that opportunistically merges the small segments produced by a
// flush into a single larger segment, reducing the number of tiny
// segments a near-real-time reader has to open. It decorates an
// underlying index.MergePolicy and only augments the flush-time merge
// decision; all other policy behaviour is delegated to the wrapped
// policy.
package index
