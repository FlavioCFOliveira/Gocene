// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "testing"

func TestConfigurationKey(t *testing.T) {
	k := NewConfigurationKey("myKey")
	if k.String() != "myKey" {
		t.Errorf("String() = %q, want %q", k.String(), "myKey")
	}
}

func TestAbstractQueryConfig_SetGet(t *testing.T) {
	k1 := NewConfigurationKey("k1")
	k2 := NewConfigurationKey("k2")
	cfg := newAbstractQueryConfig()

	if cfg.Has(k1) {
		t.Error("Has(k1) should be false before Set")
	}
	cfg.Set(k1, "hello")
	if !cfg.Has(k1) {
		t.Error("Has(k1) should be true after Set")
	}
	if v := cfg.Get(k1); v != "hello" {
		t.Errorf("Get(k1) = %v, want %q", v, "hello")
	}
	if cfg.Has(k2) {
		t.Error("Has(k2) should be false")
	}

	cfg.Set(k1, nil) // nil removes the entry
	if cfg.Has(k1) {
		t.Error("Has(k1) should be false after Set(nil)")
	}
}

func TestFieldConfig(t *testing.T) {
	fc := NewFieldConfig("body")
	if fc.GetField() != "body" {
		t.Errorf("GetField() = %q, want %q", fc.GetField(), "body")
	}
	k := NewConfigurationKey("slop")
	fc.Set(k, 3)
	if v := fc.Get(k); v != 3 {
		t.Errorf("Get(slop) = %v, want 3", v)
	}
}

func TestFieldConfig_EmptyNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty fieldName")
		}
	}()
	NewFieldConfig("")
}

func TestQueryConfigHandler_GetFieldConfig(t *testing.T) {
	h := NewQueryConfigHandler()

	k := NewConfigurationKey("boost")
	h.AddFieldConfigListener(FieldConfigListenerFunc(func(fc *FieldConfig) {
		fc.Set(k, float32(2.0))
	}))

	fc := h.GetFieldConfig("title")
	if fc == nil {
		t.Fatal("GetFieldConfig returned nil")
	}
	if fc.GetField() != "title" {
		t.Errorf("field = %q, want %q", fc.GetField(), "title")
	}
	if v := fc.Get(k); v != float32(2.0) {
		t.Errorf("boost = %v, want 2.0", v)
	}
}

func TestQueryConfigHandler_NilFieldName(t *testing.T) {
	h := NewQueryConfigHandler()
	if fc := h.GetFieldConfig(""); fc != nil {
		t.Error("GetFieldConfig(\"\") should return nil")
	}
}

// FieldConfigListenerFunc is a helper adapter for inline listener functions.
type FieldConfigListenerFunc func(fc *FieldConfig)

func (f FieldConfigListenerFunc) BuildFieldConfig(fc *FieldConfig) { f(fc) }
