package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"io"
	"io/fs"
	"net"
	"net/http"
	"sync"

	"github.com/ethereum/hive/internal/libhive"
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

	srv := &proxyServer{
		cb:              cb,
		containerID:     id,
		containerIP:     net.ParseIP(info.IP),
		containerWait:   info.Wait,
		containerStdin:  inR,
		containerStdout: outW,
		handler:         h,
		localIn:         outR,
		localOut:        inW,
	}
	srv.wg.Add(1)
	go srv.loop()
	return srv, nil
}

type proxyServer struct {
	cb libhive.ContainerBackend

	containerID     string
	containerIP     net.IP
	containerStdin  *io.PipeReader
	containerStdout *io.PipeWriter
	containerWait   func()

	handler  http.Handler
	localIn  *io.PipeReader
	localOut *io.PipeWriter

	wg       sync.WaitGroup
	stopping sync.Once
	stopErr  error
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
		s.localIn.Close()
		s.localOut.Close()
		s.wg.Wait()
	})
	return s.stopErr
}

// loop reads requests from the proxy container and runs them.
func (s *proxyServer) loop() {
	defer s.wg.Done()

	var (
		in    = bufio.NewReader(s.localIn)
		respW = new(proxyRespWriter)
	)
	for {
		req, err := http.ReadRequest(in)
		if err != nil {
			if err != io.EOF {
				log15.Error("read error in proxy", "err", err)
			}
			return
		}

		log15.Debug("serving proxy request", "method", req.Method, "url", req.URL)
		s.handler.ServeHTTP(respW, req)

		// Drain the body.
		io.Copy(io.Discard, req.Body)
		req.Body.Close()

		// Write out the response.
		if err := respW.writeTo(s.localOut); err != nil {
			log15.Error("response write error in proxy", "err", err)
			return
		}
		respW.reset()
	}
}

// proxyRespWriter is the http.ResponseWriter used by the proxy.
type proxyRespWriter struct {
	resp          http.Response
	headerWritten bool
	bodyBuf       bytes.Buffer
}

func (rw *proxyRespWriter) Header() http.Header {
	if rw.resp.Header == nil {
		rw.resp.Header = make(http.Header)
	}
	return rw.resp.Header
}

func (rw *proxyRespWriter) WriteHeader(status int) {
	if !rw.headerWritten {
		rw.resp.StatusCode = status
		rw.resp.Status = http.StatusText(status)
		rw.headerWritten = true
	}
}

func (rw *proxyRespWriter) Write(b []byte) (int, error) {
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.bodyBuf.Write(b)
}

func (rw *proxyRespWriter) writeTo(w io.Writer) error {
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	if rw.bodyBuf.Len() > 0 {
		rw.resp.Body = io.NopCloser(&rw.bodyBuf)
		rw.resp.ContentLength = int64(rw.bodyBuf.Len())
	}
	return rw.resp.Write(w)
}

func (rw *proxyRespWriter) reset() {
	header := rw.resp.Header
	for k := range header {
		delete(header, k)
	}
	rw.resp = http.Response{Header: header}
	rw.headerWritten = false
	rw.bodyBuf.Reset()
}
