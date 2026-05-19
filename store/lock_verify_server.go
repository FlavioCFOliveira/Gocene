// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Wire-protocol constants used by the lock verification server and matching
// clients (the network variant of VerifyingLockFactory). Their values are
// fixed by the upstream Lucene protocol and must not change.
const (
	// MsgLockReleased is the single-byte command a client sends to signal
	// the server it has just released the lock. It matches Lucene's
	// VerifyingLockFactory.MSG_LOCK_RELEASED.
	MsgLockReleased = 0

	// MsgLockAcquired is the single-byte command a client sends to signal
	// the server it has just acquired the lock. It matches Lucene's
	// VerifyingLockFactory.MSG_LOCK_ACQUIRED.
	MsgLockAcquired = 1

	// StartGunSignal is the single-byte signal the server writes to every
	// connected client once all clients have registered, releasing them to
	// start the lock stress run. It matches Lucene's
	// LockVerifyServer.START_GUN_SIGNAL.
	StartGunSignal = 43
)

// lockVerifyServerAcceptTimeout bounds how long the server will wait for the
// requested number of clients to connect and send their identifier byte.
// Matches the 30 seconds SO_TIMEOUT applied in Lucene's LockVerifyServer.
const lockVerifyServerAcceptTimeout = 30 * time.Second

// LockVerifyServerOnReady is invoked by [LockVerifyServerRun] once the server
// has bound its listener but before it starts accepting clients. The callback
// receives the resolved local address so tests can hand it to the clients they
// spawn. It is the Go analogue of Lucene's startClients consumer.
type LockVerifyServerOnReady func(addr net.Addr)

// LockVerifyServerRun runs the lock verification server until maxClients
// clients have connected, exchanged the wire protocol, and disconnected.
//
// The server binds a TCP listener on hostname and an ephemeral port, waits
// for maxClients to connect (each sending a one-byte identifier), fires the
// start gun, and then arbitrates lock acquire/release messages, ensuring at
// most one client holds the lock at any time. Any protocol violation aborts
// the run by closing the offending connection; remaining clients observe EOF
// and exit. The function returns when every client connection has been fully
// drained or when the bind / accept phase fails.
//
// onReady may be nil; when non-nil it is invoked after a successful bind,
// before the first Accept call.
//
// LockVerifyServerRun is the Go port of
// org.apache.lucene.store.LockVerifyServer#run.
func LockVerifyServerRun(hostname string, maxClients int, onReady LockVerifyServerOnReady) error {
	if maxClients < 0 {
		return fmt.Errorf("lock verify server: maxClients must be >= 0, got %d", maxClients)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(hostname, "0"))
	if err != nil {
		return fmt.Errorf("lock verify server: bind: %w", err)
	}
	defer listener.Close()

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return fmt.Errorf("lock verify server: unexpected listener type %T", listener)
	}

	addr := listener.Addr()
	fmt.Fprintf(os.Stdout, "Listening on %s...\n", addr)

	if onReady != nil {
		onReady(addr)
	}

	type clientConn struct {
		id   int
		conn net.Conn
	}

	clients := make([]clientConn, 0, maxClients)
	defer func() {
		for _, c := range clients {
			_ = c.conn.Close()
		}
	}()

	// Phase 1: accept all clients and read their identifier byte. Any failure
	// here aborts the run before the start gun fires.
	for accepted := 0; accepted < maxClients; accepted++ {
		if err := tcpListener.SetDeadline(time.Now().Add(lockVerifyServerAcceptTimeout)); err != nil {
			return fmt.Errorf("lock verify server: set accept deadline: %w", err)
		}

		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("lock verify server: accept client %d: %w", accepted, err)
		}

		idBuf := [1]byte{}
		if err := conn.SetReadDeadline(time.Now().Add(lockVerifyServerAcceptTimeout)); err != nil {
			_ = conn.Close()
			return fmt.Errorf("lock verify server: set read deadline on client %d: %w", accepted, err)
		}
		if _, err := io.ReadFull(conn, idBuf[:]); err != nil {
			_ = conn.Close()
			return fmt.Errorf("lock verify server: read id from client %d: %w", accepted, err)
		}
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			_ = conn.Close()
			return fmt.Errorf("lock verify server: clear read deadline on client %d: %w", accepted, err)
		}

		clients = append(clients, clientConn{id: int(idBuf[0]), conn: conn})
	}

	// Clear the accept deadline so the listener cleanup at function exit is
	// not racing a stale deadline.
	if err := tcpListener.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("lock verify server: clear accept deadline: %w", err)
	}

	// Phase 2: arbitrate. All clients have registered; release them.
	var (
		mu       sync.Mutex
		lockedID = -1 // -1 unlocked, -2 protocol error latched, >=0 holder id
		wg       sync.WaitGroup
		errs     []error
		errsMu   sync.Mutex
	)

	recordErr := func(err error) {
		errsMu.Lock()
		errs = append(errs, err)
		errsMu.Unlock()
	}

	for _, c := range clients {
		// Write the start-gun signal sequentially under the same conn before
		// detaching the handler goroutine, so the client side observes the
		// release in a deterministic order.
		if _, err := c.conn.Write([]byte{StartGunSignal}); err != nil {
			return fmt.Errorf("lock verify server: write start gun to client %d: %w", c.id, err)
		}
	}

	fmt.Fprintln(os.Stdout, "All clients started, fire gun...")

	for _, c := range clients {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.conn.Close()

			buf := [1]byte{}
			for {
				if _, err := io.ReadFull(c.conn, buf[:]); err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
						return
					}
					recordErr(fmt.Errorf("client %d: read command: %w", c.id, err))
					return
				}
				command := int(buf[0])

				mu.Lock()
				if lockedID == -2 {
					mu.Unlock()
					return // another client errored; cascade exit
				}
				switch command {
				case MsgLockAcquired:
					if lockedID != -1 {
						holder := lockedID
						lockedID = -2
						mu.Unlock()
						recordErr(fmt.Errorf("client %d got lock, but %d already holds the lock", c.id, holder))
						return
					}
					lockedID = c.id
				case MsgLockReleased:
					if lockedID != c.id {
						holder := lockedID
						lockedID = -2
						mu.Unlock()
						recordErr(fmt.Errorf("client %d released the lock, but %d is the one holding the lock", c.id, holder))
						return
					}
					lockedID = -1
				default:
					lockedID = -2
					mu.Unlock()
					recordErr(fmt.Errorf("client %d sent unrecognized command: %d", c.id, command))
					return
				}
				mu.Unlock()

				if _, err := c.conn.Write([]byte{byte(command)}); err != nil {
					recordErr(fmt.Errorf("client %d: write ack: %w", c.id, err))
					return
				}
			}
		}()
	}

	wg.Wait()

	fmt.Fprintln(os.Stdout, "Server terminated.")

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// LockVerifyServerMain is the argv-driven entry point matching Lucene's
// LockVerifyServer#main. It expects exactly two arguments — the bind IP and
// the number of clients — and returns an exit code suitable for handing to
// [os.Exit]. The caller is responsible for actually invoking os.Exit; this
// keeps the function testable.
//
// The function writes its own usage message to stderr on argument errors so
// that a thin cmd/ wrapper can stay trivial.
func LockVerifyServerMain(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: lockverifyserver bindToIp clients")
		return 1
	}

	maxClients, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock verify server: invalid client count %q: %v\n", args[1], err)
		return 1
	}

	if err := LockVerifyServerRun(args[0], maxClients, nil); err != nil {
		fmt.Fprintf(os.Stderr, "lock verify server: %v\n", err)
		return 1
	}
	return 0
}
