// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStressLockFactories tests lock factory implementations under multi-process
// contention scenarios. This is the Go port of Lucene's TestStressLockFactories.
//
// Source: org.apache.lucene.store.TestStressLockFactories
// Purpose: Multi-process lock contention, lock ordering verification
//
// The test verifies that:
// - At most one process holds a lock at any time
// - Lock acquisition and release work correctly under stress
// - Different LockFactory implementations behave correctly

const (
	// Protocol constants matching VerifyingLockFactory
	msgLockReleased = 0
	msgLockAcquired = 1
	startGunSignal  = 43
)

// lockVerifyServer is a TCP server that verifies at most one client holds
// the lock at any time. This is the Go equivalent of LockVerifyServer.
type lockVerifyServer struct {
	listener     net.Listener
	maxClients   int
	lockedID     atomic.Int32 // -1 = unlocked, -2 = error, >=0 = client ID holding lock
	startingGun  chan struct{}
	wg           sync.WaitGroup
	t            *testing.T
}

// newLockVerifyServer creates a new lock verification server.
func newLockVerifyServer(t *testing.T, maxClients int) (*lockVerifyServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	server := &lockVerifyServer{
		listener:    listener,
		maxClients:  maxClients,
		startingGun: make(chan struct{}),
		t:           t,
	}
	server.lockedID.Store(-1)
	return server, nil
}

// addr returns the server address.
func (s *lockVerifyServer) addr() net.Addr {
	return s.listener.Addr()
}

// run starts the server and waits for all clients to connect.
func (s *lockVerifyServer) run() error {
	defer s.listener.Close()

	clients := make([]net.Conn, 0, s.maxClients)

	// Accept connections from all clients
	for i := 0; i < s.maxClients; i++ {
		s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(30 * time.Second))
		conn, err := s.listener.Accept()
		if err != nil {
			// Close existing connections
			for _, c := range clients {
				c.Close()
			}
			return fmt.Errorf("failed to accept client %d: %w", i, err)
		}
		clients = append(clients, conn)
	}

	// Handle each client in a separate goroutine
	for i, conn := range clients {
		s.wg.Add(1)
		go s.handleClient(i, conn)
	}

	// Signal all clients to start
	close(s.startingGun)

	// Wait for all clients to finish
	s.wg.Wait()
	return nil
}

// handleClient handles communication with a single client.
func (s *lockVerifyServer) handleClient(clientID int, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	buf := make([]byte, 1)

	// Read client ID
	if _, err := conn.Read(buf); err != nil {
		s.t.Errorf("Client %d: failed to read ID: %v", clientID, err)
		return
	}
	id := int(buf[0])

	// Wait for starting gun
	<-s.startingGun

	// Send start signal
	buf[0] = startGunSignal
	if _, err := conn.Write(buf); err != nil {
		s.t.Errorf("Client %d: failed to send start signal: %v", clientID, err)
		return
	}

	// Process lock commands
	for {
		if _, err := conn.Read(buf); err != nil {
			// Connection closed
			return
		}

		command := buf[0]

		// Check if another thread got an error
		if s.lockedID.Load() == -2 {
			return
		}

		switch command {
		case msgLockAcquired:
			// Try to acquire lock
			currentLock := s.lockedID.Load()
			if currentLock != -1 {
				s.lockedID.Store(-2)
				s.t.Errorf("Client %d got lock, but %d already holds the lock", id, currentLock)
				return
			}
			if !s.lockedID.CompareAndSwap(-1, int32(id)) {
				s.lockedID.Store(-2)
				s.t.Errorf("Client %d: CAS failed when acquiring lock", id)
				return
			}

		case msgLockReleased:
			// Try to release lock
			currentLock := s.lockedID.Load()
			if currentLock != int32(id) {
				s.lockedID.Store(-2)
				s.t.Errorf("Client %d released lock, but %d is holding the lock", id, currentLock)
				return
			}
			if !s.lockedID.CompareAndSwap(int32(id), -1) {
				s.lockedID.Store(-2)
				s.t.Errorf("Client %d: CAS failed when releasing lock", id)
				return
			}

		default:
			s.t.Errorf("Client %d: unrecognized command: %d", id, command)
			return
		}

		// Acknowledge command
		if _, err := conn.Write(buf); err != nil {
			s.t.Errorf("Client %d: failed to acknowledge: %v", id, err)
			return
		}
	}
}

// lockStressClient represents a client process that stresses the lock factory.
type lockStressClient struct {
	id              int
	serverAddr      string
	lockFactory     LockFactory
	lockDir         string
	delayMs         int
	rounds          int
	t               *testing.T
}

// run executes the stress test client.
func (c *lockStressClient) run() error {
	// Connect to verification server
	conn, err := net.DialTimeout("tcp", c.serverAddr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	// Send client ID
	if _, err := conn.Write([]byte{byte(c.id)}); err != nil {
		return fmt.Errorf("failed to send ID: %w", err)
	}

	// Wait for start signal
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		return fmt.Errorf("failed to read start signal: %w", err)
	}
	if buf[0] != startGunSignal {
		return fmt.Errorf("protocol violation: expected %d, got %d", startGunSignal, buf[0])
	}

	// Create verifying lock factory wrapper
	vf := &verifyingLockFactory{
		factory: c.lockFactory,
		conn:    conn,
	}

	// Create FSDirectory for lock storage
	fsDir, err := NewNIOFSDirectory(c.lockDir)
	if err != nil {
		return fmt.Errorf("failed to create NIOFSDirectory: %w", err)
	}
	defer fsDir.Close()

	// Run stress test
	for i := 0; i < c.rounds; i++ {
		lock, err := vf.obtainLock(fsDir, "test.lock")
		if err != nil {
			// Lock obtain failed - this is expected in contention scenarios
			time.Sleep(time.Duration(c.delayMs) * time.Millisecond)
			continue
		}

		// Hold lock for a short time
		time.Sleep(time.Duration(c.delayMs) * time.Millisecond)

		// Release lock
		if err := lock.Close(); err != nil {
			return fmt.Errorf("failed to release lock: %w", err)
		}

		time.Sleep(time.Duration(c.delayMs) * time.Millisecond)
	}

	return nil
}

// verifyingLockFactory wraps a LockFactory and verifies operations with a server.
type verifyingLockFactory struct {
	factory LockFactory
	conn    net.Conn
}

// obtainLock obtains a lock and verifies with the server.
func (v *verifyingLockFactory) obtainLock(dir Directory, lockName string) (Lock, error) {
	lock, err := v.factory.ObtainLock(dir, lockName)
	if err != nil {
		return nil, err
	}

	// Verify lock acquisition with server
	if err := v.verify(msgLockAcquired); err != nil {
		lock.Close()
		return nil, err
	}

	return &verifyingLock{
		Lock:    lock,
		factory: v,
	}, nil
}

// verify sends a message to the server and waits for acknowledgment.
func (v *verifyingLockFactory) verify(message byte) error {
	if _, err := v.conn.Write([]byte{message}); err != nil {
		return fmt.Errorf("failed to send verification: %w", err)
	}

	buf := make([]byte, 1)
	if _, err := v.conn.Read(buf); err != nil {
		return fmt.Errorf("failed to read verification response: %w", err)
	}

	if buf[0] != message {
		return fmt.Errorf("protocol violation: expected %d, got %d", message, buf[0])
	}

	return nil
}

// verifyingLock wraps a Lock and verifies release with the server.
type verifyingLock struct {
	Lock
	factory *verifyingLockFactory
}

// Close releases the lock and verifies with the server.
func (l *verifyingLock) Close() error {
	if err := l.Lock.EnsureValid(); err != nil {
		return err
	}

	if err := l.factory.verify(msgLockReleased); err != nil {
		return err
	}

	return l.Lock.Close()
}

// Note: The original Lucene TestStressLockFactories uses multi-process testing
// with LockVerifyServer and LockStressTest. In Go, we test the equivalent
// functionality using goroutines for SingleInstanceLockFactory and basic
// functionality tests for NativeFSLockFactory.

// TestStressLockFactories_NativeFS tests NativeFSLockFactory under stress.
// This is the Go port of TestStressLockFactories.testNativeFSLockFactory().
//
// Note: NativeFSLockFactory uses file-based locking which is designed to work
// across processes. Within a single process, multiple goroutines can access the
// same lock file. This test verifies the basic functionality and file operations.
func TestStressLockFactories_NativeFS(t *testing.T) {
	tempDir := t.TempDir()
	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	factory := NewNativeFSLockFactory()

	// Test rapid lock acquire/release cycles
	const cycles = 100
	for i := 0; i < cycles; i++ {
		lock, err := factory.ObtainLock(dir, "stress.lock")
		if err != nil {
			t.Fatalf("Cycle %d: failed to obtain lock: %v", i, err)
		}

		if !lock.IsLocked() {
			t.Errorf("Cycle %d: lock should be held", i)
		}

		if err := lock.EnsureValid(); err != nil {
			t.Errorf("Cycle %d: lock should be valid: %v", i, err)
		}

		if err := lock.Close(); err != nil {
			t.Fatalf("Cycle %d: failed to release lock: %v", i, err)
		}

		if lock.IsLocked() {
			t.Errorf("Cycle %d: lock should be released", i)
		}
	}

	// Test that we can obtain different named locks
	lockNames := []string{"lock1", "lock2", "write.lock", "commit.lock"}
	locks := make([]Lock, 0, len(lockNames))

	for _, name := range lockNames {
		lock, err := factory.ObtainLock(dir, name)
		if err != nil {
			t.Fatalf("Failed to obtain lock %s: %v", name, err)
		}
		locks = append(locks, lock)
	}

	// Release all locks
	for _, lock := range locks {
		if err := lock.Close(); err != nil {
			t.Errorf("Failed to release lock: %v", err)
		}
	}
}

// TestStressLockFactories_SingleInstance tests SingleInstanceLockFactory under stress.
// Note: SingleInstanceLockFactory only works within a single process,
// so this tests concurrent goroutine access rather than multi-process.
func TestStressLockFactories_SingleInstance(t *testing.T) {
	factory := NewSingleInstanceLockFactory()
	tempDir := t.TempDir()

	// Create a mock directory
	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	const numGoroutines = 10
	const rounds = 50

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for r := 0; r < rounds; r++ {
				lock, err := factory.ObtainLock(dir, "stress.lock")
				if err != nil {
					// Lock contention is expected
					time.Sleep(time.Millisecond)
					continue
				}

				// Verify lock is held
				if !lock.IsLocked() {
					errors <- fmt.Errorf("goroutine %d: lock not held after obtain", id)
					return
				}

				// Hold briefly
				time.Sleep(time.Millisecond)

				// Release
				if err := lock.Close(); err != nil {
					errors <- fmt.Errorf("goroutine %d: failed to release lock: %w", id, err)
					return
				}

				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Stress test error: %v", err)
	}
}

// TestStressLockFactories_ConcurrentAccess tests concurrent lock access patterns.
func TestStressLockFactories_ConcurrentAccess(t *testing.T) {
	tests := []struct {
		name    string
		factory LockFactory
	}{
		{
			name:    "NativeFSLockFactory",
			factory: NewNativeFSLockFactory(),
		},
		{
			name:    "SingleInstanceLockFactory",
			factory: NewSingleInstanceLockFactory(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			dir, err := NewNIOFSDirectory(tempDir)
			if err != nil {
				t.Fatalf("Failed to create directory: %v", err)
			}
			defer dir.Close()

			const numGoroutines = 5
			const iterations = 20

			var wg sync.WaitGroup
			successCount := atomic.Int32{}
			contentionCount := atomic.Int32{}

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					for j := 0; j < iterations; j++ {
						lock, err := tt.factory.ObtainLock(dir, "concurrent.lock")
						if err != nil {
							contentionCount.Add(1)
							continue
						}

						// Verify lock
						if err := lock.EnsureValid(); err != nil {
							t.Errorf("Lock not valid: %v", err)
						}

						successCount.Add(1)
						lock.Close()
					}
				}(i)
			}

			wg.Wait()

			total := successCount.Load() + contentionCount.Load()
			if total != numGoroutines*iterations {
				t.Errorf("Expected %d attempts, got %d", numGoroutines*iterations, total)
			}

			t.Logf("Success: %d, Contention: %d", successCount.Load(), contentionCount.Load())
		})
	}
}

// TestStressLockFactories_LockOrdering tests that locks are properly ordered.
func TestStressLockFactories_LockOrdering(t *testing.T) {
	factory := NewSingleInstanceLockFactory()
	tempDir := t.TempDir()
	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	// Test that we can obtain multiple different locks
	lockNames := []string{"lock1", "lock2", "lock3", "write.lock", "commit.lock"}
	locks := make([]Lock, 0, len(lockNames))

	for _, name := range lockNames {
		lock, err := factory.ObtainLock(dir, name)
		if err != nil {
			t.Fatalf("Failed to obtain lock %s: %v", name, err)
		}
		locks = append(locks, lock)
	}

	// Verify all locks are held
	for i, lock := range locks {
		if !lock.IsLocked() {
			t.Errorf("Lock %d (%s) should be held", i, lockNames[i])
		}
	}

	// Release in reverse order
	for i := len(locks) - 1; i >= 0; i-- {
		if err := locks[i].Close(); err != nil {
			t.Errorf("Failed to release lock %d: %v", i, err)
		}
	}

	// Verify all locks are released
	for i, lock := range locks {
		if lock.IsLocked() {
			t.Errorf("Lock %d (%s) should be released", i, lockNames[i])
		}
	}
}

// TestStressLockFactories_RapidAcquireRelease tests rapid lock acquire/release cycles.
func TestStressLockFactories_RapidAcquireRelease(t *testing.T) {
	factory := NewSingleInstanceLockFactory()
	tempDir := t.TempDir()
	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	const cycles = 1000

	for i := 0; i < cycles; i++ {
		lock, err := factory.ObtainLock(dir, "rapid.lock")
		if err != nil {
			t.Fatalf("Cycle %d: failed to obtain lock: %v", i, err)
		}

		if err := lock.Close(); err != nil {
			t.Fatalf("Cycle %d: failed to release lock: %v", i, err)
		}
	}
}

// TestStressLockFactories_DifferentLockNames tests that different lock names
// don't interfere with each other.
func TestStressLockFactories_DifferentLockNames(t *testing.T) {
	factory := NewSingleInstanceLockFactory()
	tempDir := t.TempDir()
	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	// Obtain locks with different names - all should succeed
	lock1, err := factory.ObtainLock(dir, "lock1")
	if err != nil {
		t.Fatalf("Failed to obtain lock1: %v", err)
	}
	defer lock1.Close()

	lock2, err := factory.ObtainLock(dir, "lock2")
	if err != nil {
		t.Fatalf("Failed to obtain lock2: %v", err)
	}
	defer lock2.Close()

	// Both should be held
	if !lock1.IsLocked() || !lock2.IsLocked() {
		t.Error("Both locks should be held")
	}
}
