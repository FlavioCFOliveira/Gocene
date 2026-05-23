// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// slowDirectory is a test-only Directory wrapper that introduces artificial
// I/O delays. Mirrors org.apache.lucene.facet.SlowDirectory.
//
// The delay is applied after every IO_SLEEP_THRESHOLD bytes. When sleepMillis
// is -1 no delay is applied and the wrapper acts as a pure delegate.

import (
	"math"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

const ioSleepThreshold = 50

// slowDirectory wraps a store.Directory and injects sleeps after every
// ioSleepThreshold bytes transferred.
type slowDirectory struct {
	*store.FilterDirectory
	sleepMillis int
}

// newSlowDirectory creates a slowDirectory wrapping an in-memory
// ByteBuffersDirectory. sleepMillis=-1 disables all delays.
func newSlowDirectory(sleepMillis int) *slowDirectory {
	return &slowDirectory{
		FilterDirectory: store.NewFilterDirectory(store.NewByteBuffersDirectory()),
		sleepMillis:     sleepMillis,
	}
}

// SetSleepMillis updates the per-IO sleep time at runtime.
func (d *slowDirectory) SetSleepMillis(ms int) { d.sleepMillis = ms }

// doSleep sleeps for a length-adjusted duration.
func (d *slowDirectory) doSleep(length int) {
	if d.sleepMillis <= 0 {
		return
	}
	ms := d.sleepMillis
	if length >= 10 {
		ms = int(float64(d.sleepMillis) * math.Log(float64(length)))
	}
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

// CreateOutput wraps the output in a slowIndexOutput when sleepMillis != -1.
func (d *slowDirectory) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	out, err := d.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}
	if d.sleepMillis == -1 {
		return out, nil
	}
	return newSlowIndexOutput(out, d), nil
}

// OpenInput wraps the input in a slowIndexInput when sleepMillis != -1.
func (d *slowDirectory) OpenInput(name string, ctx store.IOContext) (store.IndexInput, error) {
	in, err := d.FilterDirectory.OpenInput(name, ctx)
	if err != nil {
		return nil, err
	}
	if d.sleepMillis == -1 {
		return in, nil
	}
	return newSlowIndexInput(in, d), nil
}

// ---------------------------------------------------------------------------
// slowIndexInput
// ---------------------------------------------------------------------------

type slowIndexInput struct {
	*store.FilterIndexInput
	numRead int
	dir     *slowDirectory
}

func newSlowIndexInput(in store.IndexInput, dir *slowDirectory) *slowIndexInput {
	return &slowIndexInput{
		FilterIndexInput: store.NewFilterIndexInput("SlowIndexInput", in),
		dir:              dir,
	}
}

func (s *slowIndexInput) ReadByte() (byte, error) {
	if s.numRead >= ioSleepThreshold {
		s.dir.doSleep(0)
		s.numRead = 0
	}
	s.numRead++
	return s.FilterIndexInput.ReadByte()
}

func (s *slowIndexInput) ReadBytes(b []byte) error {
	if s.numRead >= ioSleepThreshold {
		s.dir.doSleep(len(b))
		s.numRead = 0
	}
	s.numRead += len(b)
	return s.FilterIndexInput.ReadBytes(b)
}

// ---------------------------------------------------------------------------
// slowIndexOutput
// ---------------------------------------------------------------------------

type slowIndexOutput struct {
	*store.FilterIndexOutput
	numWrote int
	dir      *slowDirectory
}

func newSlowIndexOutput(out store.IndexOutput, dir *slowDirectory) *slowIndexOutput {
	return &slowIndexOutput{
		FilterIndexOutput: store.NewFilterIndexOutput(
			"SlowIndexOutput("+out.GetName()+")", out.GetName(), out),
		dir: dir,
	}
}

func (s *slowIndexOutput) WriteByte(b byte) error {
	if s.numWrote >= ioSleepThreshold {
		s.dir.doSleep(0)
		s.numWrote = 0
	}
	s.numWrote++
	return s.FilterIndexOutput.WriteByte(b)
}

func (s *slowIndexOutput) WriteBytes(b []byte) error {
	if s.numWrote >= ioSleepThreshold {
		s.dir.doSleep(len(b))
		s.numWrote = 0
	}
	s.numWrote += len(b)
	return s.FilterIndexOutput.WriteBytes(b)
}
