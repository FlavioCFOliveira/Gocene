// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeSPI is a minimal NamedSPI implementation used to exercise
// NamedSPILoader without depending on Codec.
type fakeSPI struct{ name string }

func (f fakeSPI) Name() string { return f.name }

// TestNamedSPILoader_Lookup mirrors TestNamedSPILoader.testLookup
// (Lucene 10.4.0). We replace Codec with the fakeSPI stub.
func TestNamedSPILoader_Lookup(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	if err := l.Register(fakeSPI{name: "default"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := l.Lookup("default")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.Name() != "default" {
		t.Fatalf("Lookup returned %q, want %q", got.Name(), "default")
	}
}

// TestNamedSPILoader_BogusLookup mirrors testBogusLookup. The Java
// version expects IllegalArgumentException; in Go we expect an error
// wrapping ErrSPINotFound.
func TestNamedSPILoader_BogusLookup(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	_, err := l.Lookup("dskfdskfsdfksdfdsf")
	if !errors.Is(err, ErrSPINotFound) {
		t.Fatalf("expected ErrSPINotFound, got %v", err)
	}
}

// TestNamedSPILoader_AvailableServices mirrors testAvailableServices.
func TestNamedSPILoader_AvailableServices(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := l.Register(fakeSPI{name: name}); err != nil {
			t.Fatalf("Register %q: %v", name, err)
		}
	}
	avail := l.AvailableServices()
	want := []string{"alpha", "beta", "gamma"}
	if len(avail) != len(want) {
		t.Fatalf("AvailableServices=%v, want %v", avail, want)
	}
	for i := range want {
		if avail[i] != want[i] {
			t.Fatalf("AvailableServices[%d]=%q, want %q (insertion order)", i, avail[i], want[i])
		}
	}
}

// TestNamedSPILoader_DuplicateRegistration verifies the Lucene
// "only add the first one for each name" rule.
func TestNamedSPILoader_DuplicateRegistration(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	first := fakeSPI{name: "x"}
	if err := l.Register(first); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := l.Register(fakeSPI{name: "x"}); err != nil {
		t.Fatalf("Register second: %v", err)
	}
	got, _ := l.Lookup("x")
	if got != first {
		t.Fatalf("expected first registration to win, got %+v", got)
	}
	if l.Size() != 1 {
		t.Fatalf("expected size 1, got %d", l.Size())
	}
}

// TestNamedSPILoader_Iterator verifies insertion-order iteration.
func TestNamedSPILoader_Iterator(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	for _, n := range []string{"c", "a", "b"} {
		_ = l.Register(fakeSPI{name: n})
	}
	it := l.Iterator()
	want := []string{"c", "a", "b"}
	for i, s := range it {
		if s.Name() != want[i] {
			t.Fatalf("Iterator[%d]=%q, want %q", i, s.Name(), want[i])
		}
	}
}

// TestNamedSPILoader_SortedAvailableServices verifies lexicographic
// ordering for the diagnostic helper.
func TestNamedSPILoader_SortedAvailableServices(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	for _, n := range []string{"c", "a", "b"} {
		_ = l.Register(fakeSPI{name: n})
	}
	got := l.SortedAvailableServices()
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedAvailableServices[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

// TestCheckServiceName_OK covers the validation happy path.
func TestCheckServiceName_OK(t *testing.T) {
	names := []string{"Lucene99", "abc", "ABC123", strings.Repeat("a", 127)}
	for _, n := range names {
		if err := CheckServiceName(n); err != nil {
			t.Fatalf("CheckServiceName(%q): unexpected %v", n, err)
		}
	}
}

// TestCheckServiceName_Bad covers the Java IllegalArgumentException
// branches: too long, and non ASCII-alphanumeric.
func TestCheckServiceName_Bad(t *testing.T) {
	bad := []string{
		strings.Repeat("a", 128), // exactly 128 → rejected
		"with space",
		"dash-name",
		"dot.name",
		"naïve",
	}
	for _, n := range bad {
		if err := CheckServiceName(n); !errors.Is(err, ErrInvalidSPIName) {
			t.Fatalf("CheckServiceName(%q): expected ErrInvalidSPIName, got %v", n, err)
		}
	}
}

// TestNamedSPILoader_Concurrent stresses the lock-free read /
// mutex-write path with many concurrent registrations.
func TestNamedSPILoader_Concurrent(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	var wg sync.WaitGroup
	const N = 256
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n := "svc" + itoa(i)
			if err := l.Register(fakeSPI{name: n}); err != nil {
				t.Errorf("Register %q: %v", n, err)
			}
		}(i)
	}
	wg.Wait()
	if l.Size() != N {
		t.Fatalf("Size=%d, want %d", l.Size(), N)
	}
}

// itoa is a tiny helper used by TestNamedSPILoader_Concurrent to avoid
// pulling strconv into a leaf-level test.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [16]byte{}
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestNamedSPILoader_Reload is a smoke test on the no-op so the
// callable surface stays compatible.
func TestNamedSPILoader_Reload(t *testing.T) {
	l := NewNamedSPILoader[fakeSPI]("FakeSPI")
	l.Reload()
	if l.Size() != 0 {
		t.Fatalf("Reload should leave size at 0, got %d", l.Size())
	}
}
