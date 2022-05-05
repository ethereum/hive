package main

import (
	"fmt"
	"net"

	"github.com/rauljordan/engine-proxy/proxy"
)

type Proxy struct {
	IP        net.IP
	proxy     *proxy.Proxy
	JWTSecret string
}

func NewProxy(port int, destination string) *Proxy {
	host := "localhost"
	config := proxy.SpoofingConfig{
		Requests: []*proxy.Spoof{
			&proxy.Spoof{},
		},
	}
	options := []proxy.Option{
		proxy.WithHost(host),
		proxy.WithPort(port),
		proxy.WithDestinationAddress(destination),
		proxy.WithSpootingConfig(&config),
	}
	proxy, err := proxy.New(options...)
	if err != nil {
		panic(err)
	}
	return &Proxy{IP: net.ParseIP("127.0.0.1"), proxy: proxy}
}

func (en *Proxy) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", en.IP, PortUserRPC), nil
}

func (en *Proxy) EngineRPCAddress() (string, error) {
	// TODO what will the default port be?
	return fmt.Sprintf("http://%v:%d", en.IP, PortEngineRPC), nil
}
