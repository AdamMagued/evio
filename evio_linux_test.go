// Copyright 2018 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:build linux
// +build linux

package evio

import (
	"bytes"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestLinuxEventfdListener(t *testing.T) {
	var (
		servingCalled int32
		openedCalled  int32
		wokenCalled   int32
		dataEchoed    int32
		testDone      int32
	)

	var events Events
	events.NumLoops = 2

	serverReady := make(chan struct{})
	serverDone := make(chan error, 1)

	events.Serving = func(srv Server) (action Action) {
		atomic.StoreInt32(&servingCalled, 1)
		close(serverReady)
		return
	}

	events.Opened = func(c Conn) (out []byte, opts Options, action Action) {
		atomic.StoreInt32(&openedCalled, 1)
		go func() {
			time.Sleep(20 * time.Millisecond)
			atomic.StoreInt32(&wokenCalled, 1)
			c.Wake()
		}()
		return
	}

	events.Data = func(c Conn, in []byte) (out []byte, action Action) {
		if len(in) > 0 {
			atomic.StoreInt32(&dataEchoed, 1)
			out = in
		}
		return
	}

	events.Tick = func() (delay time.Duration, action Action) {
		delay = 50 * time.Millisecond
		if atomic.LoadInt32(&testDone) == 1 {
			action = Shutdown
		}
		return
	}

	addr := "127.0.0.1:19977"
	go func() {
		serverDone <- Serve(events, "tcp://"+addr)
	}()

	select {
	case <-serverReady:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for evio server to start")
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)
	}

	payload := []byte("eventfd-cross-build-test")
	if _, err := conn.Write(payload); err != nil {
		conn.Close()
		t.Fatalf("conn.Write failed: %v", err)
	}

	reply := make([]byte, len(payload))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, reply); err != nil {
		conn.Close()
		t.Fatalf("io.ReadFull failed: %v", err)
	}

	if !bytes.Equal(reply, payload) {
		conn.Close()
		t.Fatalf("expected payload %q, got %q", payload, reply)
	}

	time.Sleep(50 * time.Millisecond)
	_ = conn.Close()

	if atomic.LoadInt32(&servingCalled) != 1 {
		t.Fatal("expected Serving event to be called")
	}
	if atomic.LoadInt32(&openedCalled) != 1 {
		t.Fatal("expected Opened event to be called")
	}
	if atomic.LoadInt32(&wokenCalled) != 1 {
		t.Fatal("expected Wake/eventfd trigger to be called")
	}
	if atomic.LoadInt32(&dataEchoed) != 1 {
		t.Fatal("expected Data event to be called")
	}

	// Signal tick to shutdown server
	atomic.StoreInt32(&testDone, 1)

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("unexpected Serve error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for evio server to shutdown")
	}
}
