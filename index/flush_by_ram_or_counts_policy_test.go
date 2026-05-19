// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Tests for FlushByRamOrCountsPolicy. The fakes (fakeDWPT, fakePool,
// fakeOwner, fakeConfig, fakeDeleteQueue) live in
// documents_writer_flush_control_test.go and are shared across the
// package-internal test suite.

// recordingInfoStream captures every (component, message) emitted so
// tests can assert that FP traces fire exactly when expected.
type recordingInfoStream struct {
	mu       sync.Mutex
	enabled  map[string]bool
	messages []recordedFPMessage
}

type recordedFPMessage struct {
	component string
	message   string
}

func newRecordingInfoStream(enabledComponents ...string) *recordingInfoStream {
	enabled := make(map[string]bool, len(enabledComponents))
	for _, c := range enabledComponents {
		enabled[c] = true
	}
	return &recordingInfoStream{enabled: enabled}
}

func (s *recordingInfoStream) IsEnabled(component string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled[component]
}

func (s *recordingInfoStream) Message(component, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, recordedFPMessage{component: component, message: message})
}

// Close satisfies io.Closer (and therefore util.InfoStream).
func (s *recordingInfoStream) Close() error { return nil }

func (s *recordingInfoStream) fpMessages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.messages))
	for _, m := range s.messages {
		if m.component == "FP" {
			out = append(out, m.message)
		}
	}
	return out
}

// newPolicyHarness constructs a FlushByRamOrCountsPolicy together with a
// fresh control, owner and pool. The policy is wired through the fakeConfig
// so OnChange reads exactly the same numbers the control sees.
func newPolicyHarness(
	t *testing.T,
	ramBufferMB float64,
	maxBufferedDocs int,
	infoStream util.InfoStream,
) (*FlushByRamOrCountsPolicy, *DocumentsWriterFlushControl, *fakeOwner, *fakePool) {
	t.Helper()
	if infoStream == nil {
		infoStream = util.NoOpInfoStream
	}
	pool := &fakePool{}
	owner := &fakeOwner{
		pool:        pool,
		deleteQueue: &fakeDeleteQueue{},
	}
	cfg := &fakeConfig{
		ramBufferMB:     ramBufferMB,
		maxBufferedDocs: maxBufferedDocs,
		hardLimitMB:     1945,
		infoStream:      infoStream,
	}
	policy := NewFlushByRamOrCountsPolicy(cfg)
	cfg.policy = policy
	control := NewDocumentsWriterFlushControl(owner, cfg)
	return policy, control, owner, pool
}

// ---------------------------------------------------------------------------
// Construction / interface contract
// ---------------------------------------------------------------------------

func TestFlushByRamOrCountsPolicy_ImplementsFlushControlPolicy(t *testing.T) {
	var _ flushControlPolicy = (*FlushByRamOrCountsPolicy)(nil)
}

func TestFlushByRamOrCountsPolicy_FlushOnDocCount(t *testing.T) {
	tests := []struct {
		name string
		max  int
		want bool
	}{
		{"disabled", DISABLE_AUTO_FLUSH, false},
		{"enabled-positive", 100, true},
		{"enabled-zero", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy, _, _, _ := newPolicyHarness(t, 16.0, tc.max, nil)
			if got := policy.FlushOnDocCount(); got != tc.want {
				t.Fatalf("FlushOnDocCount() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlushByRamOrCountsPolicy_FlushOnRAM(t *testing.T) {
	tests := []struct {
		name string
		ram  float64
		want bool
	}{
		{"disabled", float64(DISABLE_AUTO_FLUSH), false},
		{"enabled-positive", 16.0, true},
		{"enabled-zero", 0.0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy, _, _, _ := newPolicyHarness(t, tc.ram, DISABLE_AUTO_FLUSH, nil)
			if got := policy.FlushOnRAM(); got != tc.want {
				t.Fatalf("FlushOnRAM() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Doc-count branch
// ---------------------------------------------------------------------------

func TestFlushByRamOrCountsPolicy_DocCount_MarksPendingAtThreshold(t *testing.T) {
	_, control, owner, pool := newPolicyHarness(t, float64(DISABLE_AUTO_FLUSH), 5, nil)
	dwpt := registerDWPT(pool, owner, 0, 5)

	control.invokePolicyOnChangeLocked(dwpt)

	if !dwpt.IsFlushPending() {
		t.Fatalf("expected dwpt to be marked flush-pending at doc-count threshold")
	}
}

func TestFlushByRamOrCountsPolicy_DocCount_DoesNotMarkBelowThreshold(t *testing.T) {
	_, control, owner, pool := newPolicyHarness(t, float64(DISABLE_AUTO_FLUSH), 5, nil)
	dwpt := registerDWPT(pool, owner, 0, 4)

	control.invokePolicyOnChangeLocked(dwpt)

	if dwpt.IsFlushPending() {
		t.Fatalf("dwpt was marked flush-pending below doc-count threshold")
	}
}

func TestFlushByRamOrCountsPolicy_DocCount_IgnoresNilPerThread(t *testing.T) {
	_, control, _, _ := newPolicyHarness(t, float64(DISABLE_AUTO_FLUSH), 1, nil)

	// Must not panic and must not flip flushDeletes flag.
	control.invokePolicyOnChangeLocked(nil)

	if control.GetApplyAllDeletes() {
		t.Fatalf("apply-all-deletes flag set without RAM-trigger and without perThread")
	}
}

// ---------------------------------------------------------------------------
// RAM branch — active bytes path
// ---------------------------------------------------------------------------

// commitDWPT pushes the DWPT's pre-loaded bytesUsed into the control's
// activeBytes accounting. It mirrors what DoAfterDocument does for the
// happy "not yet pending" branch but skips the policy callback so a test
// can pre-populate accounting before invoking OnChange explicitly.
func commitDWPT(control *DocumentsWriterFlushControl, dwpt *fakeDWPT) {
	delta := dwpt.GetCommitLastBytesUsedDelta()
	dwpt.CommitLastBytesUsed(delta)
	control.mu.Lock()
	control.activeBytes += delta
	control.mu.Unlock()
}

func TestFlushByRamOrCountsPolicy_RAM_FlushesLargestWhenActivePlusDeletesAboveLimit(t *testing.T) {
	// 1 MB limit; deletes are negligible, active alone trips the limit.
	_, control, owner, pool := newPolicyHarness(t, 1.0, DISABLE_AUTO_FLUSH, nil)
	limit := int64(1.0 * 1024 * 1024)

	small := registerDWPT(pool, owner, 16*1024, 1)
	largest := registerDWPT(pool, owner, limit+1, 1)

	// Push activeBytes into the control's accounting without firing the
	// policy callback (we exercise OnChange directly below).
	commitDWPT(control, small)
	commitDWPT(control, largest)

	control.invokePolicyOnChangeLocked(small)

	if !largest.IsFlushPending() {
		t.Fatalf("largest dwpt should be marked flush-pending on RAM trigger")
	}
	if small.IsFlushPending() {
		t.Fatalf("small dwpt must not be marked pending: %#v", small)
	}
	if control.GetApplyAllDeletes() {
		t.Fatalf("apply-all-deletes set without deletesRAM crossing threshold")
	}
}

func TestFlushByRamOrCountsPolicy_RAM_DoesNotFlushBelowLimit(t *testing.T) {
	_, control, owner, pool := newPolicyHarness(t, 1.0, DISABLE_AUTO_FLUSH, nil)
	dwpt := registerDWPT(pool, owner, 1024, 1)
	commitDWPT(control, dwpt)

	control.invokePolicyOnChangeLocked(dwpt)

	if dwpt.IsFlushPending() {
		t.Fatalf("dwpt was marked flush-pending below RAM threshold")
	}
}

// ---------------------------------------------------------------------------
// RAM branch — delete bytes
// ---------------------------------------------------------------------------

// inflateDeleteBytes pushes the owner's bound delete queue's RAM accounting
// past the supplied threshold. The fakeDeleteQueue exposes RAMBytesUsed but
// not a setter; we replace it on the owner and re-bind the perThreads.
func inflateDeleteBytes(owner *fakeOwner, ramBytes int64) {
	owner.deleteQueue = &fakeDeleteQueue{ramBytes: ramBytes}
}

func TestFlushByRamOrCountsPolicy_RAM_AppliesDeletesWhenDeletesOnlyAboveLimit(t *testing.T) {
	_, control, owner, _ := newPolicyHarness(t, 1.0, DISABLE_AUTO_FLUSH, nil)
	inflateDeleteBytes(owner, int64(1.0*1024*1024)+1)

	// Pure-delete event: perThread is nil.
	control.invokePolicyOnChangeLocked(nil)

	if !control.GetApplyAllDeletes() {
		t.Fatalf("expected apply-all-deletes when deletes alone exceed RAM limit")
	}
}

func TestFlushByRamOrCountsPolicy_RAM_BothPathsTriggerWhenBothCrossLimit(t *testing.T) {
	_, control, owner, pool := newPolicyHarness(t, 1.0, DISABLE_AUTO_FLUSH, nil)
	limit := int64(1.0 * 1024 * 1024)

	inflateDeleteBytes(owner, limit+1)
	largest := registerDWPT(pool, owner, limit+1, 1)
	smaller := registerDWPT(pool, owner, 4096, 1)
	commitDWPT(control, largest)
	commitDWPT(control, smaller)

	control.invokePolicyOnChangeLocked(smaller)

	if !control.GetApplyAllDeletes() {
		t.Fatalf("expected apply-all-deletes when both RAM thresholds crossed")
	}
	if !largest.IsFlushPending() {
		t.Fatalf("expected largest dwpt to be marked flush-pending")
	}
}

// ---------------------------------------------------------------------------
// RAM disabled
// ---------------------------------------------------------------------------

func TestFlushByRamOrCountsPolicy_RAMDisabled_NoFlushNoDeletes(t *testing.T) {
	_, control, owner, pool := newPolicyHarness(t, float64(DISABLE_AUTO_FLUSH), DISABLE_AUTO_FLUSH, nil)
	inflateDeleteBytes(owner, 1<<30)
	dwpt := registerDWPT(pool, owner, 1<<30, 1<<10)
	commitDWPT(control, dwpt)

	control.invokePolicyOnChangeLocked(dwpt)

	if dwpt.IsFlushPending() {
		t.Fatalf("dwpt marked pending with both triggers disabled")
	}
	if control.GetApplyAllDeletes() {
		t.Fatalf("apply-all-deletes set with both triggers disabled")
	}
}

// ---------------------------------------------------------------------------
// InfoStream tracing
// ---------------------------------------------------------------------------

func TestFlushByRamOrCountsPolicy_EmitsFPTraceOnDeletesFlush(t *testing.T) {
	stream := newRecordingInfoStream("FP")
	_, control, owner, _ := newPolicyHarness(t, 1.0, DISABLE_AUTO_FLUSH, stream)
	inflateDeleteBytes(owner, int64(1.0*1024*1024)+1)

	control.invokePolicyOnChangeLocked(nil)

	msgs := stream.fpMessages()
	if len(msgs) == 0 {
		t.Fatalf("expected at least one FP trace, got none")
	}
	if !strings.Contains(msgs[0], "force apply deletes") {
		t.Fatalf("expected force-apply-deletes trace, got %q", msgs[0])
	}
}

func TestFlushByRamOrCountsPolicy_EmitsFPTraceOnActiveBytesFlush(t *testing.T) {
	stream := newRecordingInfoStream("FP")
	_, control, owner, pool := newPolicyHarness(t, 1.0, DISABLE_AUTO_FLUSH, stream)
	limit := int64(1.0 * 1024 * 1024)
	largest := registerDWPT(pool, owner, limit+1, 1)
	commitDWPT(control, largest)

	control.invokePolicyOnChangeLocked(largest)

	msgs := stream.fpMessages()
	if len(msgs) == 0 {
		t.Fatalf("expected at least one FP trace, got none")
	}
	foundTrigger := false
	for _, m := range msgs {
		if strings.Contains(m, "trigger flush") {
			foundTrigger = true
			break
		}
	}
	if !foundTrigger {
		t.Fatalf("expected 'trigger flush' FP trace, got %v", msgs)
	}
}

func TestFlushByRamOrCountsPolicy_SuppressesTracesWhenFPDisabled(t *testing.T) {
	stream := newRecordingInfoStream() // FP not enabled
	_, control, owner, _ := newPolicyHarness(t, 1.0, DISABLE_AUTO_FLUSH, stream)
	inflateDeleteBytes(owner, int64(1.0*1024*1024)+1)

	control.invokePolicyOnChangeLocked(nil)

	if msgs := stream.fpMessages(); len(msgs) != 0 {
		t.Fatalf("expected no FP traces when component disabled, got %v", msgs)
	}
}

// ---------------------------------------------------------------------------
// Concurrency smoke test
// ---------------------------------------------------------------------------

// TestFlushByRamOrCountsPolicy_ConcurrentOnChange exercises the policy
// from multiple goroutines against an active control. The policy itself
// is stateless; this test guards against accidental introduction of
// shared mutable state and against contract violations in the helpers
// it calls.
func TestFlushByRamOrCountsPolicy_ConcurrentOnChange(t *testing.T) {
	_, control, owner, pool := newPolicyHarness(t, 1.0, 4, nil)
	limit := int64(1.0 * 1024 * 1024)

	dwpts := make([]*fakeDWPT, 8)
	for i := range dwpts {
		dwpts[i] = registerDWPT(pool, owner, limit/4, 2)
		commitDWPT(control, dwpts[i])
	}

	const goroutines = 8
	const iterations = 64
	var wg sync.WaitGroup
	var pending atomic.Int32
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				idx := (seed*iterations + i) % len(dwpts)
				dwpt := dwpts[idx]
				dwpt.Lock()
				control.invokePolicyOnChangeLocked(dwpt)
				dwpt.Unlock()
				if dwpt.IsFlushPending() {
					pending.Add(1)
				}
			}
		}(g)
	}
	wg.Wait()

	if pending.Load() == 0 {
		t.Fatalf("expected at least one flush-pending mark under load")
	}
}
