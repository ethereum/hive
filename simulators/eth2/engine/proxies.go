package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rauljordan/engine-proxy/proxy"
)

type Proxy struct {
	IP        net.IP
	port      int
	proxy     *proxy.Proxy
	JWTSecret string
	config    *proxy.SpoofingConfig

	rpc *rpc.Client
	mu  sync.Mutex
}

func NewProxy(hostIP net.IP, port int, destination string, jwtSecret []byte) *Proxy {
	host := "0.0.0.0"
	config := proxy.SpoofingConfig{}
	options := []proxy.Option{
		proxy.WithHost(host),
		proxy.WithPort(port),
		proxy.WithDestinationAddress(destination),
		proxy.WithSpoofingConfig(&config),
		proxy.WithJWTSecret(jwtSecret),
	}
	proxy, err := proxy.New(options...)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := proxy.Start(context.Background()); err != nil {
			panic(err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	log.Info("Starting new proxy", "host", host, "port", port)
	return &Proxy{IP: hostIP, port: port, proxy: proxy, config: &config}
}

func (p *Proxy) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, p.port), nil
}

func (p *Proxy) EngineRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, p.port), nil
}

func (p *Proxy) AddRequest(spoof *proxy.Spoof) {
	p.config.Requests = append(p.config.Requests, spoof)
	p.proxy.UpdateSpoofingConfig(p.config)
}

func (p *Proxy) AddResponse(spoof *proxy.Spoof) {
	p.config.Responses = append(p.config.Responses, spoof)
	p.proxy.UpdateSpoofingConfig(p.config)
}

func (p *Proxy) RPC() *rpc.Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rpc == nil {
		p.rpc, _ = rpc.DialHTTP(fmt.Sprintf("http://%v:%d", p.IP, p.port))
	}
	return p.rpc
}
