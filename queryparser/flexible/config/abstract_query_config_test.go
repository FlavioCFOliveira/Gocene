package config

import (
	"testing"
)

func TestAbstractQueryConfig(t *testing.T) {
	config := NewAbstractQueryConfig()
	key1 := NewConfigurationKey()
	key2 := NewConfigurationKey()

	t.Run("SetAndGet", func(t *testing.T) {
		val := "test-value"
		err := config.Set(key1, val)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		got, err := config.Get(key1)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got != val {
			t.Errorf("expected %v, got %v", val, got)
		}
	})

	t.Run("Has", func(t *testing.T) {
		if !config.Has(key1) {
			t.Error("expected key1 to be present")
		}
		if config.Has(key2) {
			t.Error("expected key2 to be absent")
		}
	})

	t.Run("Unset", func(t *testing.T) {
		if !config.Unset(key1) {
			t.Error("expected Unset to return true for present key")
		}
		if config.Has(key1) {
			t.Error("expected key1 to be absent after Unset")
		}
		if config.Unset(key1) {
			t.Error("expected Unset to return false for absent key")
		}
	})

	t.Run("SetNil", func(t *testing.T) {
		config.Set(key2, "value")
		config.Set(key2, nil)
		if config.Has(key2) {
			t.Error("expected key2 to be absent after setting to nil")
		}
	})

	t.Run("NullKey", func(t *testing.T) {
		err := config.Set(nil, "value")
		if err != ErrKeyMustNotBeNull {
			t.Errorf("expected ErrKeyMustNotBeNull, got %v", err)
		}

		_, err = config.Get(nil)
		if err != ErrKeyMustNotBeNull {
			t.Errorf("expected ErrKeyMustNotBeNull, got %v", err)
		}

		if config.Has(nil) {
			t.Error("Has(nil) should return false")
		}

		if config.Unset(nil) {
			t.Error("Unset(nil) should return false")
		}
	})
}
