package config

import (
	"sync/atomic"
)

var keyCounter uint64

// ConfigurationKey represents a key used to retrieve a value from AbstractQueryConfig.
// In Lucene, this is a type-safe key. In Go, the type safety is handled at the call site
// via casting or generic helpers.
type ConfigurationKey struct {
	id uint64
}

// NewConfigurationKey creates a new, unique instance of ConfigurationKey.
func NewConfigurationKey() ConfigurationKey {
	return ConfigurationKey{
		id: atomic.AddUint64(&keyCounter, 1),
	}
}
