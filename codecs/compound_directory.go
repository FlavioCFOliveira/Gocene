// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

var (
	// ErrUnsupportedOperation is returned when an operation is not supported by the compound directory.
	ErrUnsupportedOperation = errors.New("unsupported operation: not supported by CFS")
)

// CompoundDirectory is a read-only Directory that consists of a view over a compound file.
//
// Mirrors org.apache.lucene.codecs.CompoundDirectory in Apache Lucene 10.5.0.
// The declaration itself lives in the spi package because index also names this
// contract (index.CompoundDirectory), and index is imported by codecs; hosting
// it here directly would close an index <-> codecs import cycle.
type CompoundDirectory = spi.CompoundDirectory

// BaseCompoundDirectory provides a base implementation for CompoundDirectory,
// enforcing the read-only nature of compound files.
type BaseCompoundDirectory struct {
	store.BaseDirectory
}

// NewBaseCompoundDirectory creates a new BaseCompoundDirectory.
func NewBaseCompoundDirectory(lockFactory store.LockFactory) *BaseCompoundDirectory {
	return &BaseCompoundDirectory{
		BaseDirectory: *store.NewBaseDirectory(lockFactory),
	}
}

// DeleteFile is not supported by CFS.
func (d *BaseCompoundDirectory) DeleteFile(name string) error {
	return ErrUnsupportedOperation
}

// Rename is not supported by CFS.
func (d *BaseCompoundDirectory) Rename(from, to string) error {
	return ErrUnsupportedOperation
}

// CreateOutput is not supported by CFS.
func (d *BaseCompoundDirectory) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	return nil, ErrUnsupportedOperation
}

// ObtainLock is not supported by CFS.
func (d *BaseCompoundDirectory) ObtainLock(name string) (store.Lock, error) {
	return nil, ErrUnsupportedOperation
}

// CheckIntegrity is an abstract method in the original Java class.
// In this base implementation, it returns an error to ensure it is overridden by concrete implementations.
func (d *BaseCompoundDirectory) CheckIntegrity() error {
	return errors.New("CheckIntegrity not implemented in BaseCompoundDirectory")
}
