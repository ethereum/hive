//go:build !linux
// +build !linux

package main

import (
	"net"

	"gopkg.in/inconshreveable/log15.v2"
)

func bindToDevice(l net.Listener, device string) error {
	log15.Error("bindToDevice is not supported on this platform")
	return nil
}
