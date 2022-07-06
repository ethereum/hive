package hiveproxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestProxyHTTP(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()

	done := make(chan struct{})
	defer close(done)

	// Run the frontend.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		pc := RunFrontend(cr, cw, l)
		<-done
		pc.Close()
	}()

	// Run the backend.
	var called bool
	test := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	ps := RunBackend(sr, sw, test)
	defer ps.Close()

	// Send a HTTP request to frontend.
	http.Get("http://" + l.Addr().String())

	// It should have arrived in backend.
	if !called {
		t.Fatal("handler not called")
	}
}

func TestProxyCheckLive(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()

	done := make(chan struct{})
	defer close(done)

	// Run the frontend.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		pc := RunFrontend(cr, cw, l)
		<-done
		pc.Close()
	}()

	// Run the backend.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	ps := RunBackend(sr, sw, h)
	defer ps.Close()

	// Create a listener that will be hit by checkLive.
	tl, err := runTestListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tl.Close()

	// Run CheckLive on the backend side.
	if err := ps.CheckLive(context.Background(), tl.l.Addr().String()); err != nil {
		t.Fatal("CheckLive did not work:", err)
	}
}

func TestProxyCheckLiveCancel(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()

	done := make(chan struct{})
	defer close(done)

	// Run the frontend.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		pc := RunFrontend(cr, cw, l)
		<-done
		pc.Close()
	}()

	// Run the backend.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	ps := RunBackend(sr, sw, h)
	defer ps.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 400 * time.Millisecond)
	defer cancel()

	// Run CheckLive on the backend side.
	err = ps.CheckLive(ctx, "127.0.0.1:44491")
	if err == nil {
		t.Fatal("CheckLive did not return error")
	}
	t.Log(err)
}

type testListener struct {
	l  net.Listener
	wg sync.WaitGroup
}

func runTestListener(addr string) (*testListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	tl := &testListener{l: l}
	tl.wg.Add(1)
	go tl.acceptLoop()
	return tl, nil
}

func (tl *testListener) Close() {
	tl.l.Close()
	tl.wg.Wait()
}

func (tl *testListener) acceptLoop() {
	defer tl.wg.Done()
	for {
		c, err := tl.l.Accept()
		if err != nil {
			return
		}
		c.Close()
	}
}
