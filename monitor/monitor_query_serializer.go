// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"encoding/binary"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MonitorQuerySerializer serializes and deserializes MonitorQuery objects to
// and from byte streams. Use this for persistent query indexes.
//
// Port of org.apache.lucene.monitor.MonitorQuerySerializer.
//
// Wire format (binary):
//
//	[4 bytes] query ID length (n)
//	[n bytes] query ID (UTF-8)
//	[4 bytes] query string length (m)
//	[m bytes] query string (UTF-8)
//	[4 bytes] metadata entry count (k)
//	For each metadata entry:
//	  [4 bytes] key length
//	  [key bytes] key (UTF-8)
//	  [4 bytes] value length
//	  [value bytes] value (UTF-8)
type MonitorQuerySerializer interface {
	// Deserialize reconstructs a MonitorQuery from its byte representation.
	Deserialize(binaryValue *util.BytesRef) (*MonitorQuery, error)

	// Serialize converts a MonitorQuery into its byte representation.
	Serialize(query *MonitorQuery) (*util.BytesRef, error)
}

// DefaultMonitorQuerySerializer is the standard implementation of
// MonitorQuerySerializer using a simple length-prefixed binary format.
type DefaultMonitorQuerySerializer struct{}

// NewDefaultMonitorQuerySerializer creates a new serializer.
func NewDefaultMonitorQuerySerializer() *DefaultMonitorQuerySerializer {
	return &DefaultMonitorQuerySerializer{}
}

// Serialize encodes a MonitorQuery into a binary byte representation.
// The parsed Query object is not serialized (only the query string and
// metadata are persisted); the caller must re-parse after deserialization.
func (s *DefaultMonitorQuerySerializer) Serialize(mq *MonitorQuery) (*util.BytesRef, error) {
	if mq == nil {
		return nil, fmt.Errorf("MonitorQuerySerializer: cannot serialize nil query")
	}

	id := mq.GetID()
	qs := mq.GetQueryString()
	meta := mq.GetMetadata()

	// Calculate total size.
	size := 4 + len(id) + 4 + len(qs) + 4
	for k, v := range meta {
		size += 4 + len(k) + 4 + len(v)
	}

	buf := make([]byte, 0, size)

	// Query ID.
	buf = appendUint32(buf, uint32(len(id)))
	buf = append(buf, id...)

	// Query string.
	buf = appendUint32(buf, uint32(len(qs)))
	buf = append(buf, qs...)

	// Metadata entries.
	buf = appendUint32(buf, uint32(len(meta)))
	for k, v := range meta {
		buf = appendUint32(buf, uint32(len(k)))
		buf = append(buf, k...)
		buf = appendUint32(buf, uint32(len(v)))
		buf = append(buf, v...)
	}

	return util.NewBytesRef(buf), nil
}

// Deserialize reconstructs a MonitorQuery from its binary representation.
func (s *DefaultMonitorQuerySerializer) Deserialize(binaryValue *util.BytesRef) (*MonitorQuery, error) {
	if binaryValue == nil || len(binaryValue.Bytes) == 0 {
		return nil, fmt.Errorf("MonitorQuerySerializer: cannot deserialize empty value")
	}

	b := binaryValue.Bytes
	pos := 0

	// Query ID.
	idLen, err := readUint32(b, &pos)
	if err != nil {
		return nil, err
	}
	if pos+int(idLen) > len(b) {
		return nil, fmt.Errorf("MonitorQuerySerializer: truncated ID at position %d", pos)
	}
	id := string(b[pos : pos+int(idLen)])
	pos += int(idLen)

	// Query string.
	strLen, err := readUint32(b, &pos)
	if err != nil {
		return nil, err
	}
	if pos+int(strLen) > len(b) {
		return nil, fmt.Errorf("MonitorQuerySerializer: truncated query string at position %d", pos)
	}
	queryString := string(b[pos : pos+int(strLen)])
	pos += int(strLen)

	// Metadata.
	metaCount, err := readUint32(b, &pos)
	if err != nil {
		return nil, err
	}
	metadata := make(map[string]string, int(metaCount))
	for i := uint32(0); i < metaCount; i++ {
		keyLen, err := readUint32(b, &pos)
		if err != nil || pos+int(keyLen) > len(b) {
			return nil, fmt.Errorf("MonitorQuerySerializer: truncated metadata key at position %d", pos)
		}
		key := string(b[pos : pos+int(keyLen)])
		pos += int(keyLen)

		valLen, err := readUint32(b, &pos)
		if err != nil || pos+int(valLen) > len(b) {
			return nil, fmt.Errorf("MonitorQuerySerializer: truncated metadata value at position %d", pos)
		}
		val := string(b[pos : pos+int(valLen)])
		pos += int(valLen)

		metadata[key] = val
	}

	return &MonitorQuery{
		id:          id,
		query:       nil, // re-parsed by caller
		queryString: queryString,
		metadata:    metadata,
	}, nil
}

// appendUint32 appends a uint32 in big-endian order.
func appendUint32(buf []byte, v uint32) []byte {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], v)
	return append(buf, tmp[:]...)
}

// readUint32 reads a big-endian uint32 from b at *pos, advancing *pos.
func readUint32(b []byte, pos *int) (uint32, error) {
	if *pos+4 > len(b) {
		return 0, fmt.Errorf("MonitorQuerySerializer: unexpected end of data at position %d", *pos)
	}
	v := binary.BigEndian.Uint32(b[*pos:])
	*pos += 4
	return v, nil
}

// Ensure DefaultMonitorQuerySerializer implements MonitorQuerySerializer.
var _ MonitorQuerySerializer = (*DefaultMonitorQuerySerializer)(nil)
