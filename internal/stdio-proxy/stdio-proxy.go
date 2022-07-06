package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/hashicorp/yamux"
)

func main() {
	addrFlag := flag.String("addr", ":8081", "listening address")
	flag.Parse()

	mux, _ := yamux.Client(stdioRW{}, nil)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return mux.Open()
		},
	}
	proxy := httputil.ReverseProxy{
		Director:  func(req *http.Request) {},
		Transport: transport,
	}
	http.ListenAndServe(*addrFlag, &proxy)
}

type stdioRW struct{}

func (stdioRW) Read(b []byte) (int, error)  { return os.Stdin.Read(b) }
func (stdioRW) Write(b []byte) (int, error) { return os.Stdout.Write(b) }
func (stdioRW) Close() error                { return nil }
