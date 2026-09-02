// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// The codec envelope helpers (magic, header, footer) live in package spi as
// of rmp #4706 so that the lifted *spi.SegmentInfos and any future
// codec-facing code can share them without depending on package index.
// This file preserves the historical lowercase identifiers so the rest of
// package index continues to compile unchanged.

// codecMagic mirrors spi.CodecMagic.
const codecMagic int32 = spi.CodecMagic

// footerMagic mirrors spi.FooterMagic.
const footerMagic int32 = spi.FooterMagic

// writeIndexHeader is a thin forwarder to spi.WriteIndexHeader.
func writeIndexHeader(out store.IndexOutput, codec string, version int32, id []byte, suffix string) error {
	return spi.WriteIndexHeader(out, codec, version, id, suffix)
}

// checkIndexHeader is a thin forwarder to spi.CheckIndexHeader.
func checkIndexHeader(in store.IndexInput, codec string, minVersion, maxVersion int32, expectedID []byte, expectedSuffix string) (int32, error) {
	return spi.CheckIndexHeader(in, codec, minVersion, maxVersion, expectedID, expectedSuffix)
}

// writeFooter is a thin forwarder to spi.WriteFooter.
func writeFooter(out *store.ChecksumIndexOutput) error {
	return spi.WriteFooter(out)
}

// checkFooter is a thin forwarder to spi.CheckFooter.
func checkFooter(in *store.ChecksumIndexInput) (int64, error) {
	return spi.CheckFooter(in)
}
