package hiveproxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/hashicorp/yamux"
)

type Proxy struct {
	httpsrv    http.Server
	serverDown chan struct{}
	closeOnce  sync.Once
}

func newProxy() *Proxy {
	return &Proxy{
		serverDown: make(chan struct{}),
	}
}

func (p *Proxy) Close() {
	p.closeOnce.Do(func() {
		p.httpsrv.Shutdown(context.Background())
		<-p.serverDown
	})
}

func (p *Proxy) serve(l net.Listener) {
	p.httpsrv.Serve(l)
	close(p.serverDown)
}

func RunClient(r io.Reader, w io.WriteCloser, listener net.Listener) *Proxy {
	mux, _ := yamux.Client(rwCombo{r, w}, nil)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return mux.Open()
		},
	}
	p := newProxy()
	p.httpsrv.Handler = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// This is needed to satisy the transport."
			req.URL.Scheme = "http"
			req.URL.Host = "hive"
		},
		Transport: transport,
	}
	go p.serve(listener)
	return p
}

func RunServer(r io.Reader, w io.WriteCloser, h http.Handler) *Proxy {
	mux, _ := yamux.Server(rwCombo{r, w}, nil)
	p := newProxy()
	p.httpsrv.Handler = h
	go p.serve(mux)
	return p
}

type rwCombo struct {
	io.Reader
	io.WriteCloser
}
