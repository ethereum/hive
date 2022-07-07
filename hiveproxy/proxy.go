// Package hiveproxy implements the hive API server proxy.
// This is for internal use by the 'hive' tool.
//
// hiveproxy is responsible for relaying HTTP requests originating in a private docker
// network to the hive controller, which usually runs outside of Docker. The proxy
// frontend accepts the requests, and relays them to the backend over the stdio streams of
// the proxy container.
//
// The frontend also has auxiliary functions which can be triggered by the backend via
// RPC. Specifically, it can run TCP endpoint probes, which are used by hive to confirm
// that the client container has started.
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

// CheckLive instructs the proxy frontend to probe the given network address. It returns a
// non-nil error when a TCP connection to the address was successfully established.
//
// This can only be called on the proxy side created by RunBackend.
func (p *Proxy) CheckLive(ctx context.Context, addr *net.TCPAddr) error {
	if p.isFront {
		return errors.New("CheckLive called on proxy frontend")
	}

	id := atomic.AddUint64(&p.callID, 1)

	// Set up cancellation relay.
	checkDone := make(chan struct{})
	cancelDone := p.relayCancel(ctx, checkDone, id)
	defer func() {
		close(checkDone)
		<-cancelDone
	}()

	return p.rpc.CallContext(ctx, nil, "proxy_checkLive", id, addr.String())
}

// relayCancel notifies the proxy front-end when an RPC action is canceled.
func (p *Proxy) relayCancel(ctx context.Context, done <-chan struct{}, id uint64) chan struct{} {
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

// RunFrontent starts the proxy front-end, i.e. the server which accepts HTTP connections.
// HTTP is served on the given listener, and all requests which arrive there are forwarded
// to the backend.
//
// All communication with the backend runs over the given r,w streams.
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

// RunBackend starts the proxy backend, i.e. the side which handles HTTP requests proxied
// by the frontend.
//
// All communication with the frontend runs over the given r,w streams.
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
