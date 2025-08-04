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
	addr := tl.lis.Addr().(*net.TCPAddr)
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

func TestProxyWait(t *testing.T) {
	p := runProxyPair(t, nil)

	unblock := make(chan struct{})
	go func() {
		p.front.Wait()
		close(unblock)
	}()

	p.back.Close()

	select {
	case <-unblock:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not unblock")
	}
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
	frontStarted := make(chan error, 1)
	go func() {
		var err error
		p.front, err = RunFrontend(cr, cw, l)
		frontStarted <- err
	}()

	// Run the backend.
	p.back, err = RunBackend(sr, sw, h)
	if err != nil {
		<-frontStarted
		if p.front != nil {
			p.front.Close()
		}
		t.Fatal(err)
	}

	if err := <-frontStarted; err != nil {
		p.back.Close()
		t.Fatal(err)
	}
	return p
}

func (p proxyPair) close() {
	p.back.Close()
	p.front.Close()
}

type testListener struct {
	lis net.Listener
	wg  sync.WaitGroup
}

func runTestListener(addr string) (*testListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	tl := &testListener{lis: l}
	tl.wg.Add(1)
	go tl.acceptLoop()
	return tl, nil
}

func (tl *testListener) Close() {
	tl.lis.Close()
	tl.wg.Wait()
}

func (tl *testListener) acceptLoop() {
	defer tl.wg.Done()
	for {
		c, err := tl.lis.Accept()
		if err != nil {
			return
		}
		c.Close()
	}
}
