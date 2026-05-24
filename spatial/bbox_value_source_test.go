// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spatial"
)

func TestBBoxValueSourceConstruction(t *testing.T) {
	ctx := spatial.NewSpatialContext()
	strategy, err := spatial.NewBBoxStrategy("bbox", ctx)
	if err != nil {
		t.Fatalf("NewBBoxStrategy failed: %v", err)
	}
	vs := spatial.NewBBoxValueSource(strategy)
	if vs == nil {
		t.Fatal("expected non-nil BBoxValueSource")
	}
}

func TestBBoxValueSourceString(t *testing.T) {
	ctx := spatial.NewSpatialContext()
	strategy, _ := spatial.NewBBoxStrategy("myfield", ctx)
	vs := spatial.NewBBoxValueSource(strategy)
	s := vs.String()
	if !strings.Contains(s, "myfield") {
		t.Fatalf("expected String() to contain 'myfield', got %q", s)
	}
}

func TestBBoxValueSourceNilStrategy(t *testing.T) {
	vs := spatial.NewBBoxValueSource(nil)
	if got := vs.String(); got != "bboxShape()" {
		t.Fatalf("expected 'bboxShape()', got %q", got)
	}
}

func TestBBoxValueSourceGetValues(t *testing.T) {
	vs := spatial.NewBBoxValueSource(nil)
	v, err := vs.GetValues(nil)
	if err != nil || v != nil {
		t.Fatalf("expected nil, nil; got %v, %v", v, err)
	}
}

func TestBBoxValueSourceEquals(t *testing.T) {
	ctx := spatial.NewSpatialContext()
	s, _ := spatial.NewBBoxStrategy("f", ctx)
	vs1 := spatial.NewBBoxValueSource(s)
	vs2 := spatial.NewBBoxValueSource(s)
	if !vs1.Equals(vs2) {
		t.Fatal("expected Equals true for same strategy")
	}
}

func TestBBoxValueSourceNotEquals(t *testing.T) {
	ctx := spatial.NewSpatialContext()
	s1, _ := spatial.NewBBoxStrategy("f1", ctx)
	s2, _ := spatial.NewBBoxStrategy("f2", ctx)
	vs1 := spatial.NewBBoxValueSource(s1)
	vs2 := spatial.NewBBoxValueSource(s2)
	if vs1.Equals(vs2) {
		t.Fatal("expected Equals false for different strategies")
	}
}

func TestBBoxValueSourceGetStrategy(t *testing.T) {
	ctx := spatial.NewSpatialContext()
	s, _ := spatial.NewBBoxStrategy("f", ctx)
	vs := spatial.NewBBoxValueSource(s)
	if vs.GetStrategy() != s {
		t.Fatal("expected GetStrategy to return the original strategy pointer")
	}
}
