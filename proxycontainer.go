package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/ethereum/hive/hiveproxy"
	"github.com/ethereum/hive/internal/libhive"
)

const hiveproxyTag = "hive-proxy"

func buildProxy(ctx context.Context, builder libhive.Builder) error {
	return builder.BuildImage(ctx, hiveproxyTag, hiveproxy.Source)
}

func startProxy(ctx context.Context, cb libhive.ContainerBackend, h http.Handler) (*proxyContainer, error) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	opts := libhive.ContainerOptions{Output: outW, Input: inR}
	id, err := cb.CreateContainer(ctx, hiveproxyTag, opts)
	if err != nil {
		return nil, err
	}
	info, err := cb.StartContainer(ctx, id, opts)
	if err != nil {
		cb.DeleteContainer(id)
		return nil, err
	}

	proxy := hiveproxy.RunBackend(outR, inW, h)
	srv := &proxyContainer{
		cb:              cb,
		containerID:     id,
		containerIP:     net.ParseIP(info.IP),
		containerWait:   info.Wait,
		containerStdin:  inR,
		containerStdout: outW,
		proxy:           proxy,
	}
	return srv, nil
}

type proxyContainer struct {
	cb libhive.ContainerBackend

	containerID     string
	containerIP     net.IP
	containerStdin  *io.PipeReader
	containerStdout *io.PipeWriter
	containerWait   func()
	proxy           *hiveproxy.Proxy

	stopping sync.Once
	stopErr  error
}

// addr returns the listening address of the proxy server.
func (s *proxyContainer) addr() *net.TCPAddr {
	return &net.TCPAddr{IP: s.containerIP, Port: 8081}
}

// stop terminates the proxy container and loop.
func (s *proxyContainer) stop() error {
	s.stopping.Do(func() {
		s.containerStdin.Close()
		s.containerStdout.Close()
		s.stopErr = s.cb.DeleteContainer(s.containerID)
		s.containerWait()

		s.proxy.Close()
	})
	return s.stopErr
}
