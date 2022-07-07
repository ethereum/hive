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
	var called bool
	test := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	p := runProxyPair(t, test)
	defer p.close()

	// Send a HTTP request to frontend.
	http.Get("http://" + p.lis.Addr().String())

	// It should have arrived in backend.
	if !called {
		t.Fatal("handler not called")
	}
}

func TestProxyCheckLive(t *testing.T) {
	p := runProxyPair(t, nil)
	defer p.close()

	// Create a listener that will be hit by checkLive.
	tl, err := runTestListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tl.Close()

	// Run CheckLive on the backend side.
	addr := tl.l.Addr().(*net.TCPAddr)
	if err := p.back.CheckLive(context.Background(), addr); err != nil {
		t.Fatal("CheckLive did not work:", err)
	}
}

func TestProxyCheckLiveCancel(t *testing.T) {
	p := runProxyPair(t, nil)
	defer p.close()

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	// Run CheckLive on the backend side.
	err := p.back.CheckLive(ctx, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 44491})
	if err == nil {
		t.Fatal("CheckLive did not return error")
	}
	t.Log(err)
}

type proxyPair struct {
	front *Proxy
	back  *Proxy
	lis   net.Listener
}

func runProxyPair(t *testing.T, h http.Handler) proxyPair {
	t.Helper()

	var p proxyPair
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()

	// Run the frontend.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p.lis = l
	frontStarted := make(chan struct{})
	go func() {
		p.front = RunFrontend(cr, cw, l)
		close(frontStarted)
	}()

	// Run the backend.
	p.back = RunBackend(sr, sw, h)
	<-frontStarted
	return p
}

func (p proxyPair) close() {
	p.back.Close()
	p.front.Close()
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
