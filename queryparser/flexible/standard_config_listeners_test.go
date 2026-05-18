// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"testing"
	"time"
)

func TestFieldBoostMapFCListener(t *testing.T) {
	boostMap := map[string]float32{"title": 2.0, "body": 1.0}
	listener := NewFieldBoostMapFCListener(boostMap)

	fc := NewFieldConfig("title")
	listener.BuildFieldConfig(fc)
	v := fc.Get(FieldConfigBoostKey)
	if v == nil {
		t.Fatal("expected boost set on title")
	}
	if v.(float32) != 2.0 {
		t.Errorf("boost = %v, want 2.0", v)
	}

	// Field not in map
	fc2 := NewFieldConfig("other")
	listener.BuildFieldConfig(fc2)
	if fc2.Get(FieldConfigBoostKey) != nil {
		t.Error("unexpected boost on unmapped field")
	}

	// nil fieldConfig: must not panic
	listener.BuildFieldConfig(nil)

	// Verify GetFieldBoostMap
	m := listener.GetFieldBoostMap()
	if len(m) != 2 {
		t.Errorf("GetFieldBoostMap len = %d", len(m))
	}

	// SetFieldBoostMap
	listener.SetFieldBoostMap(map[string]float32{"x": 5.0})
	fc3 := NewFieldConfig("x")
	listener.BuildFieldConfig(fc3)
	if fc3.Get(FieldConfigBoostKey).(float32) != 5.0 {
		t.Error("new map not applied")
	}
}

func TestFieldDateResolutionFCListener(t *testing.T) {
	listener := NewFieldDateResolutionFCListener(24 * time.Hour)

	// Default resolution applied to all fields
	fc := NewFieldConfig("created")
	listener.BuildFieldConfig(fc)
	v := fc.Get(FieldConfigDateResolutionKey)
	if v == nil {
		t.Fatal("expected date resolution set")
	}
	if v.(time.Duration) != 24*time.Hour {
		t.Errorf("resolution = %v, want 24h", v)
	}

	// Per-field override
	listener.SetFieldDateResolution("updated", time.Hour)
	fc2 := NewFieldConfig("updated")
	listener.BuildFieldConfig(fc2)
	if fc2.Get(FieldConfigDateResolutionKey).(time.Duration) != time.Hour {
		t.Error("per-field override not applied")
	}

	// nil fieldConfig: must not panic
	listener.BuildFieldConfig(nil)
}

func TestPointsConfigListener(t *testing.T) {
	pc := NewPointsConfig(PointsTypeInt, 1)
	m := map[string]*PointsConfig{"price": pc}
	listener := NewPointsConfigListener(m)

	fc := NewFieldConfig("price")
	listener.BuildFieldConfig(fc)
	v := fc.Get(FieldConfigPointsConfigKey)
	if v == nil {
		t.Fatal("expected PointsConfig set on price")
	}
	if v.(*PointsConfig) != pc {
		t.Error("wrong PointsConfig")
	}

	// Field not in map
	fc2 := NewFieldConfig("other")
	listener.BuildFieldConfig(fc2)
	if fc2.Get(FieldConfigPointsConfigKey) != nil {
		t.Error("unexpected PointsConfig on unmapped field")
	}

	// nil fieldConfig: must not panic
	listener.BuildFieldConfig(nil)

	// nil map constructor
	l2 := NewPointsConfigListener(nil)
	if l2.GetPointsConfigMap() == nil {
		t.Error("map should be non-nil even when nil passed to constructor")
	}
}
