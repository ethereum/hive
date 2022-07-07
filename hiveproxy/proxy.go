// Package hiveproxy implements the hive API server proxy.
// This is for internal use by the 'hive' tool.
//
// The proxy is responsible for relaying HTTP requests originating in a private docker
// network to the hive controller, which usually runs outside of Docker. The proxy
// front-end accepts the requests, and relays them to the back-end over the stdio streams
// of the proxy container.
//
// The proxy front-end also has auxiliary functions which can be triggered by the
// back-end. Specifically, the proxy can probe TCP endpoints to check if they are alive.
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

var muxcfg *yamux.Config

func init() {
	muxcfg = yamux.DefaultConfig()
	muxcfg.EnableKeepAlive = false
	muxcfg.ConnectionWriteTimeout = 30 * time.Second
}

// Proxy represents a running proxy server.
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

// Close terminates the proxy.
func (p *Proxy) Close() {
	p.closeOnce.Do(func() {
		p.httpsrv.Close()
		p.rpc.Close()
		<-p.serverDown
	})
}

// CheckLive instructs the proxy front-end to probe the given network address. It returns
// a non-nil error when a TCP connection to the address was successfully established.
//
// This can only be called on the proxy side created by RunBackend.
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
		select {
		case <-ctx.Done():
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			p.rpc.Notify(bgCtx, "proxy_cancel", id)
			cancel()
		case <-done:
		}
		close(cancelDone)
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

// RunFrontent starts the proxy front-end, i.e. the server which accepts HTTP
// connections. HTTP is served on the given listener, and all requests which arrive
// there are forwarded to the back-end.
//
// All communication with the back-end runs over the given r,w streams.
func RunFrontend(r io.Reader, w io.WriteCloser, listener net.Listener) *Proxy {
	mux, _ := yamux.Client(rwCombo{r, w}, muxcfg)
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

// RunBackend starts the proxy back-end, i.e. the side which handles HTTP requests
// proxied by the front-end.
//
// All communication with the front-end runs over the given r,w streams.
func RunBackend(r io.Reader, w io.WriteCloser, h http.Handler) *Proxy {
	mux, _ := yamux.Server(rwCombo{r, w}, muxcfg)
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
