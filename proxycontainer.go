package main

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/hive/internal/libhive"
	"github.com/hashicorp/yamux"
	"gopkg.in/inconshreveable/log15.v2"
)

//go:embed internal/stdio-proxy
var stdioProxyFS embed.FS

const stdioProxyTag = "hive-stdio-proxy"

func buildProxy(ctx context.Context, builder libhive.Builder) error {
	root, err := fs.Sub(stdioProxyFS, "internal/stdio-proxy")
	if err != nil {
		return err
	}
	return builder.BuildImage(ctx, stdioProxyTag, root)
}

func startProxy(ctx context.Context, cb libhive.ContainerBackend, h http.Handler) (*proxyServer, error) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	opts := libhive.ContainerOptions{Output: outW, Input: inR}
	id, err := cb.CreateContainer(ctx, stdioProxyTag, opts)
	if err != nil {
		return nil, err
	}
	info, err := cb.StartContainer(ctx, id, opts)
	if err != nil {
		cb.DeleteContainer(id)
		return nil, err
	}

	mux, _ := yamux.Server(rwCombo{Reader: outR, Writer: inW}, nil)
	httpsrv := &http.Server{
		Handler: h,
		ConnState: func(c net.Conn, st http.ConnState) {
			log15.Debug("proxy conn state change", "conn", c, "state", st)
		},
	}
	srv := &proxyServer{
		cb:              cb,
		containerID:     id,
		containerIP:     net.ParseIP(info.IP),
		containerWait:   info.Wait,
		containerStdin:  inR,
		containerStdout: outW,
		httpsrv:         httpsrv,
		serverDown:      make(chan error, 1),
	}
	go func() {
		srv.serverDown <- httpsrv.Serve(mux)
	}()

	return srv, nil
}

type proxyServer struct {
	cb libhive.ContainerBackend

	containerID     string
	containerIP     net.IP
	containerStdin  *io.PipeReader
	containerStdout *io.PipeWriter
	containerWait   func()

	httpsrv *http.Server

	serverDown chan error
	stopping   sync.Once
	stopErr    error
}

// addr returns the listening address of the proxy server.
func (s *proxyServer) addr() *net.TCPAddr {
	return &net.TCPAddr{IP: s.containerIP, Port: 8081}
}

// stop terminates the proxy container and loop.
func (s *proxyServer) stop() error {
	s.stopping.Do(func() {
		s.containerStdin.Close()
		s.containerStdout.Close()
		s.stopErr = s.cb.DeleteContainer(s.containerID)
		s.containerWait()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.httpsrv.Shutdown(ctx)
		<-s.serverDown
	})
	return s.stopErr
}

type rwCombo struct {
	io.Reader
	io.Writer
}

func (rwCombo) Close() error { return nil }
