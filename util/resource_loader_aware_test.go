// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"testing"
)

type fakeAware struct {
	informed bool
	err      error
}

func (f *fakeAware) Inform(loader ResourceLoader) error {
	f.informed = true
	return f.err
}

func TestResourceLoaderAware_InterfaceContract(t *testing.T) {
	var _ ResourceLoaderAware = (*fakeAware)(nil)
}

func TestInformAll_HappyPath(t *testing.T) {
	a := &fakeAware{}
	b := &fakeAware{}
	loader := NewClasspathResourceLoader(nil, nil)
	if err := InformAll(loader, a, b); err != nil {
		t.Fatalf("InformAll: %v", err)
	}
	if !a.informed || !b.informed {
		t.Fatalf("expected both informed; got a=%v b=%v", a.informed, b.informed)
	}
}

func TestInformAll_StopsOnError(t *testing.T) {
	want := errors.New("boom")
	a := &fakeAware{err: want}
	b := &fakeAware{}
	loader := NewClasspathResourceLoader(nil, nil)
	if err := InformAll(loader, a, b); !errors.Is(err, want) {
		t.Fatalf("got %v; want %v", err, want)
	}
	if b.informed {
		t.Fatalf("second component must not be informed after error")
	}
}

func TestInformAll_NilSkipped(t *testing.T) {
	a := &fakeAware{}
	loader := NewClasspathResourceLoader(nil, nil)
	if err := InformAll(loader, nil, a, nil); err != nil {
		t.Fatalf("InformAll: %v", err)
	}
	if !a.informed {
		t.Fatalf("expected a informed")
	}
}
