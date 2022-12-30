package proxy

import (
	"context"
	"encoding/json"
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

func NewProxy(
	hostIP net.IP,
	port int,
	destination string,
	jwtSecret []byte,
) *Proxy {
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
	return &Proxy{
		IP:        hostIP,
		port:      port,
		proxy:     proxy,
		config:    &config,
		callbacks: &callbacks,
		cancel:    cancel,
	}
}

func (p *Proxy) Cancel() error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}
func (p *Proxy) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, p.port), nil
}

func (p *Proxy) EngineRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", p.IP, p.port), nil
}

func (p *Proxy) AddRequestCallback(
	method string,
	callback func([]byte) *proxy.Spoof,
) {
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

func (p *Proxy) AddResponseCallback(
	method string,
	callback func([]byte, []byte) *proxy.Spoof,
) {
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

func Combine(a, b *proxy.Spoof) *proxy.Spoof {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Method != b.Method {
		panic(
			fmt.Errorf(
				"spoof methods don't match: %s != %s",
				a.Method,
				b.Method,
			),
		)
	}
	for k, v := range b.Fields {
		a.Fields[k] = v
	}
	return a
}

// Json helpers
// A value of this type can a JSON-RPC request, notification, successful response or
// error response. Which one it is depends on the fields.
type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func UnmarshalFromJsonRPCResponse(b []byte, result interface{}) error {
	var rpcMessage jsonrpcMessage
	err := json.Unmarshal(b, &rpcMessage)
	if err != nil {
		return err
	}
	if rpcMessage.Error != nil {
		return rpcMessage.Error
	}
	return json.Unmarshal(rpcMessage.Result, &result)
}

func UnmarshalFromJsonRPCRequest(b []byte, params ...interface{}) error {
	var rpcMessage jsonrpcMessage
	err := json.Unmarshal(b, &rpcMessage)
	if err != nil {
		return err
	}
	if rpcMessage.Error != nil {
		return rpcMessage.Error
	}
	return json.Unmarshal(rpcMessage.Params, &params)
}
