// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// NoDeletionPolicy is an IndexDeletionPolicy that retains every index commit
// (it never deletes). Mirrors
// org.apache.lucene.index.NoDeletionPolicy from Apache Lucene 10.4.0.
//
// The Java original is a singleton accessed via NoDeletionPolicy.INSTANCE.
// Gocene exposes the same singleton via the NoDeletionPolicyInstance package
// variable.
type NoDeletionPolicy struct{}

// NoDeletionPolicyInstance is the canonical singleton instance.
var NoDeletionPolicyInstance IndexDeletionPolicy = &NoDeletionPolicy{}

// OnInit is a no-op.
func (n *NoDeletionPolicy) OnInit(_ []*IndexCommit) error { return nil }

// OnCommit is a no-op.
func (n *NoDeletionPolicy) OnCommit(_ []*IndexCommit) error { return nil }

// Clone returns the same singleton; NoDeletionPolicy has no state.
func (n *NoDeletionPolicy) Clone() IndexDeletionPolicy { return n }
