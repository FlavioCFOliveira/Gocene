// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

type scopeNode struct {
	parent Scope
}

func (s *scopeNode) Parent() Scope { return s.parent }

func TestIsParentScope_SelfIsItsOwnParent(t *testing.T) {
	t.Parallel()

	root := &scopeNode{}
	if !IsParentScope(root, root) {
		t.Errorf("IsParentScope(root, root) = false, want true")
	}
}

func TestIsParentScope_DirectParent(t *testing.T) {
	t.Parallel()

	root := &scopeNode{}
	child := &scopeNode{parent: root}
	if !IsParentScope(root, child) {
		t.Errorf("IsParentScope(root, child) = false, want true")
	}
	if IsParentScope(child, root) {
		t.Errorf("IsParentScope(child, root) = true, want false")
	}
}

func TestIsParentScope_GrandParent(t *testing.T) {
	t.Parallel()

	root := &scopeNode{}
	mid := &scopeNode{parent: root}
	leaf := &scopeNode{parent: mid}
	if !IsParentScope(root, leaf) {
		t.Errorf("IsParentScope(root, leaf) = false, want true")
	}
	if !IsParentScope(mid, leaf) {
		t.Errorf("IsParentScope(mid, leaf) = false, want true")
	}
}

func TestIsParentScope_Unrelated(t *testing.T) {
	t.Parallel()

	a := &scopeNode{}
	b := &scopeNode{}
	if IsParentScope(a, b) {
		t.Errorf("IsParentScope(a, b) = true, want false")
	}
}

func TestIsParentScope_NilArguments(t *testing.T) {
	t.Parallel()

	root := &scopeNode{}
	if IsParentScope(nil, root) {
		t.Errorf("IsParentScope(nil, root) = true, want false")
	}
	if IsParentScope(root, nil) {
		t.Errorf("IsParentScope(root, nil) = true, want false")
	}
	if IsParentScope(nil, nil) {
		t.Errorf("IsParentScope(nil, nil) = true, want false")
	}
}
