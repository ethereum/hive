package hiveproxy

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestCheckLiveInterval(t *testing.T) {
	tests := []struct {
		elapsed time.Duration
		want    time.Duration
	}{
		{0, 5 * time.Millisecond},
		{199 * time.Millisecond, 5 * time.Millisecond},
		{200 * time.Millisecond, 10 * time.Millisecond},
		{1499 * time.Millisecond, 10 * time.Millisecond},
		{1500 * time.Millisecond, 50 * time.Millisecond},
		{4499 * time.Millisecond, 50 * time.Millisecond},
		{4500 * time.Millisecond, 100 * time.Millisecond},
		{time.Hour, 100 * time.Millisecond},
	}
	for _, test := range tests {
		if got := checkLiveInterval(test.elapsed); got != test.want {
			t.Errorf("checkLiveInterval(%v) = %v, want %v", test.elapsed, got, test.want)
		}
	}
}

// This checks that CheckLive returns after the initial delay when the port is
// already open.
func TestCheckLiveOpen(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	var pfn proxyFunctions
	start := time.Now()
	if err := pfn.CheckLive(context.Background(), 1, l.Addr().String()); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < checkLiveDelay || d > checkLiveDelay+50*time.Millisecond {
		t.Fatalf("CheckLive took %v on an open port, want about %v", d, checkLiveDelay)
	}
}

// This checks that CheckLive detects a port that opens while it is waiting.
func TestCheckLiveDelayed(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	const openAfter = 80 * time.Millisecond
	go func() {
		time.Sleep(openAfter)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Error(err)
			return
		}
		t.Cleanup(func() { l.Close() })
	}()

	var pfn proxyFunctions
	start := time.Now()
	if err := pfn.CheckLive(context.Background(), 1, addr); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < openAfter || d > openAfter+50*time.Millisecond {
		t.Fatalf("CheckLive returned after %v, want about %v", d, openAfter)
	}
}
