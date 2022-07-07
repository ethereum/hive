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
	callbacks *proxy.SpoofingCallbacks
	cancel    context.CancelFunc

	rpc *rpc.Client
	mu  sync.Mutex
}

func NewProxy(hostIP net.IP, port int, destination string, jwtSecret []byte) *Proxy {
	host := "0.0.0.0"
	config := proxy.SpoofingConfig{}
	callbacks := proxy.SpoofingCallbacks{
		RequestCallbacks:  make(map[string]func([]byte) *proxy.Spoof),
		ResponseCallbacks: make(map[string]func([]byte, []byte) *proxy.Spoof),
	}
	options := []proxy.Option{
		proxy.WithHost(host),
		proxy.WithPort(port),
		proxy.WithDestinationAddress(destination),
		proxy.WithSpoofingConfig(&config),
		proxy.WithSpoofingCallbacks(&callbacks),
		proxy.WithJWTSecret(jwtSecret),
	}
	proxy, err := proxy.New(options...)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := proxy.Start(ctx); err != nil && err != context.Canceled {
			panic(err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	log.Info("Starting new proxy", "host", host, "port", port)
	return &Proxy{IP: hostIP, port: port, proxy: proxy, config: &config, callbacks: &callbacks, cancel: cancel}
}

func (p *Proxy) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, p.port), nil
}

func (p *Proxy) EngineRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, p.port), nil
}

func (p *Proxy) AddRequestCallback(method string, callback func([]byte) *proxy.Spoof) {
	log.Info("Adding request spoof callback", "method", method)
	requestCallbacks := p.callbacks.RequestCallbacks
	requestCallbacks[method] = callback
	p.callbacks.RequestCallbacks = requestCallbacks
	p.proxy.UpdateSpoofingCallbacks(p.callbacks)
}

func (p *Proxy) AddRequest(spoof *proxy.Spoof) {
	log.Info("Adding spoof request", "method", spoof.Method)
	p.config.Requests = append(p.config.Requests, spoof)
	p.proxy.UpdateSpoofingConfig(p.config)
}

func (p *Proxy) AddResponseCallback(method string, callback func([]byte, []byte) *proxy.Spoof) {
	log.Info("Adding response spoof callback", "method", method)
	responseCallbacks := p.callbacks.ResponseCallbacks
	responseCallbacks[method] = callback
	p.callbacks.ResponseCallbacks = responseCallbacks
	p.proxy.UpdateSpoofingCallbacks(p.callbacks)
}

func (p *Proxy) AddResponse(spoof *proxy.Spoof) {
	log.Info("Adding spoof response", "method", spoof.Method)
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
