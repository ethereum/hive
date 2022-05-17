//go:build !linux
// +build !linux

package main

import (
	"errors"
	"net"
)

func bindToDevice(l net.Listener, device string) error {
	log15.Error("bindToDevice is not supported on this platform")
	return nil
}
