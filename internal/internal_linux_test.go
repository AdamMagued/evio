// Copyright 2017 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:build linux
// +build linux

package internal

import (
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestEventfd(t *testing.T) {
	fd, err := Eventfd()
	if err != nil {
		t.Fatalf("Eventfd() failed: %v", err)
	}
	defer syscall.Close(fd)

	if fd < 0 {
		t.Fatalf("expected non-negative fd, got %d", fd)
	}

	var val uint64 = 42
	buf := (*(*[8]byte)(unsafe.Pointer(&val)))[:]
	n, err := syscall.Write(fd, buf)
	if err != nil {
		t.Fatalf("syscall.Write failed: %v", err)
	}
	if n != 8 {
		t.Fatalf("expected 8 bytes written, got %d", n)
	}

	var readVal uint64
	readBuf := (*(*[8]byte)(unsafe.Pointer(&readVal)))[:]
	n, err = syscall.Read(fd, readBuf)
	if err != nil {
		t.Fatalf("syscall.Read failed: %v", err)
	}
	if n != 8 {
		t.Fatalf("expected 8 bytes read, got %d", n)
	}
	if readVal != 42 {
		t.Fatalf("expected read value 42, got %d", readVal)
	}
}

func TestOpenPoll(t *testing.T) {
	p := OpenPoll()
	if p == nil {
		t.Fatal("OpenPoll returned nil")
	}
	if p.fd < 0 {
		t.Fatalf("invalid epoll fd: %d", p.fd)
	}
	if p.wfd < 0 {
		t.Fatalf("invalid wake fd: %d", p.wfd)
	}

	received := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		err := p.Wait(func(fd int, note interface{}) error {
			if s, ok := note.(string); ok {
				if s == "stop" {
					return syscall.EINTR
				}
				received <- s
			}
			return nil
		})
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)

	if err := p.Trigger("hello-eventfd"); err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}

	select {
	case msg := <-received:
		if msg != "hello-eventfd" {
			t.Fatalf("expected note 'hello-eventfd', got %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for eventfd note")
	}

	if err := p.Trigger("stop"); err != nil {
		t.Fatalf("Trigger stop failed: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != syscall.EINTR {
			t.Fatalf("unexpected Wait error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for poll loop termination")
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestPollTriggerMultiple(t *testing.T) {
	p := OpenPoll()
	defer p.Close()

	const count = 5
	receivedCount := 0
	done := make(chan struct{})

	go func() {
		_ = p.Wait(func(fd int, note interface{}) error {
			if _, ok := note.(int); ok {
				receivedCount++
				if receivedCount == count {
					close(done)
					return syscall.EINTR
				}
			}
			return nil
		})
	}()

	time.Sleep(20 * time.Millisecond)

	for i := 0; i < count; i++ {
		if err := p.Trigger(i); err != nil {
			t.Fatalf("Trigger(%d) failed: %v", i, err)
		}
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("expected %d notes, got %d", count, receivedCount)
	}
}
