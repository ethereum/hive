package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rauljordan/engine-proxy/proxy"
)

type Proxy struct {
	IP        net.IP
	proxy     *proxy.Proxy
	JWTSecret string
	config    *proxy.SpoofingConfig

	rpc *rpc.Client
	mu  sync.Mutex
}

func NewProxy(port int, destination string) *Proxy {
	host := "localhost"
	config := proxy.SpoofingConfig{}
	options := []proxy.Option{
		proxy.WithHost(host),
		proxy.WithPort(port),
		proxy.WithDestinationAddress(destination),
		proxy.WithSpoofingConfig(&config),
	}
	proxy, err := proxy.New(options...)
	if err != nil {
		panic(err)
	}
	return &Proxy{IP: net.ParseIP("127.0.0.1"), proxy: proxy, config: &config}
}

func (p *Proxy) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, PortUserRPC), nil
}

func (p *Proxy) EngineRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, PortEngineRPC), nil
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
		p.rpc, _ = rpc.DialHTTP(fmt.Sprintf("http://%v:8545", p.IP))
	}
	return p.rpc
}
