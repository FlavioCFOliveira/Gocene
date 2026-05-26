// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"sync"
)

// ErrNoCodec is returned when an operation that requires a Codec is invoked
// while no Codec has been wired into the configuration or the DocumentsWriter.
//
// The default codec is normally installed by blank-importing the codec
// bridge package, e.g.:
//
//	import _ "github.com/FlavioCFOliveira/Gocene/internal/codecbridge"
//
// which calls RegisterDefaultCodec during its init. Callers that build an
// IndexWriterConfig without the blank import (or without explicitly calling
// IndexWriterConfig.SetCodec) will surface this error from any code path
// that attempts to flush documents.
var ErrNoCodec = errors.New("index: no default codec registered; blank-import internal/codecbridge or call IndexWriterConfig.SetCodec")

// defaultCodecRegistry is the process-wide registry for the default Codec.
//
// It is intentionally minimal: a single slot guarded by an RWMutex. The
// registry exists because the index/ package cannot import codecs/
// (codecs/ imports index/). The codec bridge in internal/codecbridge/
// performs the registration during its init so production callers obtain
// the real Lucene 10.4 codec by default.
type defaultCodecRegistry struct {
	mu    sync.RWMutex
	codec Codec
}

var defaultCodecReg defaultCodecRegistry

// RegisterDefaultCodec installs the process-wide default Codec used by
// NewIndexWriterConfig. It is safe to call concurrently and may be invoked
// more than once; the most recent value wins.
//
// This is normally called from an init() in a bridge package that imports
// both index/ and codecs/.
func RegisterDefaultCodec(c Codec) {
	defaultCodecReg.mu.Lock()
	defaultCodecReg.codec = c
	defaultCodecReg.mu.Unlock()
}

// GetDefaultCodec returns the process-wide default Codec previously
// installed via RegisterDefaultCodec, or nil if no default has been
// registered.
//
// Callers that require a non-nil codec should treat a nil return value as
// programmer error (ErrNoCodec) at the call site.
func GetDefaultCodec() Codec {
	defaultCodecReg.mu.RLock()
	defer defaultCodecReg.mu.RUnlock()
	return defaultCodecReg.codec
}
