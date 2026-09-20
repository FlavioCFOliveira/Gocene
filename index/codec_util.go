// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// The codec envelope helpers (magic, header, footer) live in package spi as
// of rmp #4706 so that the lifted *spi.SegmentInfos and any future
// codec-facing code can share them without depending on package index.
// This file preserves the historical lowercase identifiers so the rest of
// package index continues to compile unchanged.
//
// The header helpers forward straight to spi because store.IndexOutput and
// store.IndexInput are aliases of the spi interfaces. The footer helpers
// cannot forward: store.ChecksumIndexOutput/ChecksumIndexInput are distinct
// concrete types from spi.ChecksumIndexOutput/ChecksumIndexInput, so the
// footer body is expressed here against the narrow surface
// org.apache.lucene.codecs.CodecUtil.writeFooter/checkFooter actually uses.

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

// checksumOutput is the write surface CodecUtil.writeFooter needs: the two
// fixed-width writers plus the running checksum. Both
// *store.ChecksumIndexOutput and *spi.ChecksumIndexOutput satisfy it.
type checksumOutput interface {
	WriteInt(i int32) error
	WriteLong(i int64) error
	GetChecksum() uint32
}

// checksumInput is the read surface CodecUtil.checkFooter needs: the two
// fixed-width readers plus the running checksum. Both
// *store.ChecksumIndexInput and *spi.ChecksumIndexInput satisfy it.
type checksumInput interface {
	ReadInt() (int32, error)
	ReadLong() (int64, error)
	GetChecksum() uint32
}

// writeFooter writes the codec footer: FooterMagic (int32) + algo=0 (int32) +
// CRC32 checksum (int64).
// Mirrors org.apache.lucene.codecs.CodecUtil.writeFooter.
func writeFooter(out *store.ChecksumIndexOutput) error {
	return writeFooterTo(out)
}

// writeFooterTo is the body of writeFooter, shared by every checksum-carrying
// output shape in the module.
func writeFooterTo(out checksumOutput) error {
	if err := out.WriteInt(footerMagic); err != nil {
		return err
	}
	if err := out.WriteInt(0); err != nil { // algo = CRC32
		return err
	}
	checksum := out.GetChecksum()
	return out.WriteLong(int64(checksum))
}

// checkFooter validates the codec footer and returns the checksum. in must be
// positioned just before the footer (the FooterMagic field).
// Mirrors org.apache.lucene.codecs.CodecUtil.checkFooter.
func checkFooter(in *store.ChecksumIndexInput) (int64, error) {
	return checkFooterOf(in)
}

// checkFooterOf is the body of checkFooter, shared by every checksum-carrying
// input shape in the module.
func checkFooterOf(in checksumInput) (int64, error) {
	magic, err := in.ReadInt()
	if err != nil {
		return 0, err
	}
	if magic != footerMagic {
		return 0, fmt.Errorf("checkFooter: invalid footer magic 0x%x (expected 0x%x)", magic, footerMagic)
	}
	algo, err := in.ReadInt()
	if err != nil {
		return 0, err
	}
	if algo != 0 {
		return 0, fmt.Errorf("checkFooter: unknown checksum algorithm %d", algo)
	}
	actualChecksum := in.GetChecksum()
	expectedChecksum, err := in.ReadLong()
	if err != nil {
		return 0, err
	}
	if int64(actualChecksum) != expectedChecksum {
		return 0, fmt.Errorf("checkFooter: checksum mismatch (actual 0x%x, expected 0x%x)", actualChecksum, expectedChecksum)
	}
	return expectedChecksum, nil
}
