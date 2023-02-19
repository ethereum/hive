package libdocker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"gopkg.in/inconshreveable/log15.v2"

	"github.com/ethereum/hive/hiveproxy"
	"github.com/ethereum/hive/internal/libhive"
)

const hiveproxyTag = "hive/hiveproxy"

// Build builds the metrics setup and hiveproxy image.
func (cb *ContainerBackend) Build(ctx context.Context, b libhive.Builder) error {
	if err := cb.BuildMetrics(ctx, b); err != nil {
		return fmt.Errorf("failed to build metrics image: %w", err)
	}
	return b.BuildImage(ctx, hiveproxyTag, hiveproxy.Source)
}

// ServeAPI starts the API server.
func (cb *ContainerBackend) ServeAPI(ctx context.Context, h http.Handler) (libhive.APIServer, error) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	opts := libhive.ContainerOptions{Output: outW, Input: inR}
	id, err := cb.CreateContainer(ctx, hiveproxyTag, opts)
	if err != nil {
		return nil, err
	}

	// Launch the proxy server before starting the container.
	var (
		proxy     *hiveproxy.Proxy
		proxyErrC = make(chan error, 1)
	)
	go func() {
		var err error
		proxy, err = hiveproxy.RunBackend(outR, inW, h)
		if err != nil {
			log15.Error("proxy backend startup failed", "err", err)
		}
		proxyErrC <- err
	}()

	// Now start the container.
	info, err := cb.StartContainer(ctx, id, opts)
	if err != nil {
		cb.DeleteContainer(id)
		return nil, err
	}

	// Proxy server should come up.
	select {
	case err := <-proxyErrC:
		if err != nil {
			cb.DeleteContainer(id)
			return nil, err
		}
	}

	// Register proxy in ContainerBackend, so it can be used for CheckLive.
	cb.proxy = proxy

	srv := &proxyContainer{
		cb:              cb,
		containerID:     id,
		containerIP:     net.ParseIP(info.IP),
		containerWait:   info.Wait,
		containerStdin:  inR,
		containerStdout: outW,
		proxy:           proxy,
	}

	// Register proxy in ContainerBackend, so it can be used for CheckLive.
	cb.proxy = proxy
	log15.Info("hiveproxy started", "container", id[:12], "addr", srv.Addr())
	return srv, nil
}

type proxyContainer struct {
	cb *ContainerBackend

	containerID     string
	containerIP     net.IP
	containerStdin  *io.PipeReader
	containerStdout *io.PipeWriter
	containerWait   func()
	proxy           *hiveproxy.Proxy

	stopping sync.Once
	stopErr  error
}

// Addr returns the listening address of the proxy server.
func (c *proxyContainer) Addr() net.Addr {
	return &net.TCPAddr{IP: c.containerIP, Port: 8081}
}

// Stop terminates the proxy container.
func (c *proxyContainer) Close() error {
	c.stopping.Do(func() {
		// Unregister proxy in backend.
		c.cb.proxy = nil

		// Stop the container.
		c.containerStdin.Close()
		c.containerStdout.Close()
		c.stopErr = c.cb.DeleteContainer(c.containerID)
		c.containerWait()

		// Stop the local HTTP receiver.
		c.proxy.Close()
	})
	return c.stopErr
}
