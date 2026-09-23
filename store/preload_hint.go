// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "github.com/FlavioCFOliveira/Gocene/spi"

// PreloadHint is the Go port of org.apache.lucene.store.PreloadHint.
//
// The type lives in package spi (which breaks the store/index import cycle)
// and is aliased here so that there is exactly one Go type and its value
// satisfies FileOpenHint.
type PreloadHint = spi.PreloadHint

// PreloadInstance is the singleton PreloadHint value (PreloadHint.INSTANCE).
var PreloadInstance = spi.PreloadInstance
