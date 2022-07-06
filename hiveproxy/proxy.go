package hiveproxy

import (
	"context"
	"embed"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/hashicorp/yamux"
)

//go:embed Dockerfile go.mod go.sum *.go tool/*.go
var Source embed.FS

type Proxy struct {
	httpsrv    http.Server
	rpc        *rpc.Client
	serverDown chan struct{}
	closeOnce  sync.Once

	isFront bool
	callID  uint64
}

func newProxy(front bool) *Proxy {
	return &Proxy{
		serverDown: make(chan struct{}),
		isFront:    front,
	}
}

func (p *Proxy) Close() {
	p.closeOnce.Do(func() {
		p.httpsrv.Shutdown(context.Background())
		<-p.serverDown
	})
}

func (p *Proxy) CheckLive(ctx context.Context, addr string) error {
	if p.isFront {
		return errors.New("CheckLive called on proxy frontend")
	}

	id := atomic.AddUint64(&p.callID, 1)
	checkDone := make(chan struct{})
	cancelDone := p.backgroundCancelRPC(ctx, checkDone, id)
	defer func() {
		close(checkDone)
		<-cancelDone
	}()

	return p.rpc.CallContext(ctx, nil, "proxy_checkLive", id, addr)
}

func (p *Proxy) backgroundCancelRPC(ctx context.Context, done <-chan struct{}, id uint64) chan struct{} {
	cancelDone := make(chan struct{})
	go func() {
		defer close(cancelDone)
		select {
		case <-ctx.Done():
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			p.rpc.Notify(bgCtx, "proxy_cancel", id)
			cancel()
		case <-done:
		}
	}()
	return cancelDone
}

func (p *Proxy) serve(l net.Listener) {
	p.httpsrv.Serve(l)
	close(p.serverDown)
}

func (p *Proxy) launchRPC(stream net.Conn) {
	p.rpc, _ = rpc.DialIO(context.Background(), stream, stream)
}

func RunFrontend(r io.Reader, w io.WriteCloser, listener net.Listener) *Proxy {
	mux, _ := yamux.Client(rwCombo{r, w}, nil)
	p := newProxy(true)

	// Launch RPC handler.
	rpcConn, err := mux.Open()
	if err != nil {
		panic(err)
	}
	p.launchRPC(rpcConn)
	p.rpc.RegisterName("proxy", new(proxyFunctions))

	// Launch reverse proxy server.
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return mux.Open()
		},
	}
	p.httpsrv.Handler = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// This is needed to make http.Transport forward the request.
			req.URL.Scheme = "http"
			req.URL.Host = "hive"
		},
		Transport: transport,
	}
	go p.serve(listener)
	return p
}

func RunBackend(r io.Reader, w io.WriteCloser, h http.Handler) *Proxy {
	mux, _ := yamux.Server(rwCombo{r, w}, nil)
	p := newProxy(false)

	// Start RPC client.
	rpcConn, err := mux.Accept()
	if err != nil {
		panic(err)
	}
	p.launchRPC(rpcConn)

	// Start HTTP server.
	p.httpsrv.Handler = h
	go p.serve(mux)
	return p
}

type rwCombo struct {
	io.Reader
	io.WriteCloser
}
