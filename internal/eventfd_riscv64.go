// Copyright 2017 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:build linux && riscv64
// +build linux,riscv64

package internal

import "syscall"

func eventfd() (int, error) {
	r0, _, e0 := syscall.Syscall(syscall.SYS_EVENTFD2, 0, 0, 0)
	if e0 == 0 {
		return int(r0), nil
	}
	return -1, e0
}
