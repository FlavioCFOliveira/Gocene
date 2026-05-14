// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"io"
	"testing"
)

// stubResourceLoader is a fixed-in-memory ResourceLoader used only to
// pin the interface contract.
type stubResourceLoader struct{}

func (stubResourceLoader) OpenResource(name string) (io.ReadCloser, error) {
	return nil, ErrResourceNotFound
}
func (stubResourceLoader) FindFactory(name string) (FactoryFunc, error) {
	return nil, ErrFactoryNotFound
}
func (stubResourceLoader) NewInstance(name string) (any, error) {
	return nil, ErrFactoryNotFound
}

// TestResourceLoader_ContractPinned is a compile-time pin: it fails
// to build if the ResourceLoader interface signature ever drifts.
func TestResourceLoader_ContractPinned(t *testing.T) {
	var l ResourceLoader = stubResourceLoader{}
	if _, err := l.OpenResource("x"); err != ErrResourceNotFound {
		t.Fatalf("expected ErrResourceNotFound, got %v", err)
	}
	if _, err := l.FindFactory("x"); err != ErrFactoryNotFound {
		t.Fatalf("expected ErrFactoryNotFound, got %v", err)
	}
	if _, err := l.NewInstance("x"); err != ErrFactoryNotFound {
		t.Fatalf("expected ErrFactoryNotFound, got %v", err)
	}
}

// TestResourceLoader_ImplementersExist verifies the two production
// implementers (ClasspathResourceLoader and ModuleResourceLoader)
// satisfy the interface.
func TestResourceLoader_ImplementersExist(t *testing.T) {
	var _ ResourceLoader = (*ClasspathResourceLoader)(nil)
	var _ ResourceLoader = (*ModuleResourceLoader)(nil)
}
