//go:build linux
// +build linux

package main

import (
	"net"
	"syscall"
)

func bindToDevice(l net.Listener, device string) error {
	raw, err := l.(*net.TCPListener).SyscallConn()
	if err != nil {
		return err
	}
	var bindErr error
	doit := func(fd uintptr) { bindErr = syscall.BindToDevice(int(fd), device) }
	if err := raw.Control(doit); err != nil {
		return err
	}
	return bindErr
}
