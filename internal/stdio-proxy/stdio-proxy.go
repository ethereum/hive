package main

import (
	"bufio"
	"flag"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"sync"
)

func main() {
	addrFlag := flag.String("addr", ":8081", "listening address")
	flag.Parse()
	rt := newRoundTripper(os.Stdin, os.Stdout)
	proxy := httputil.ReverseProxy{
		Director:  func(req *http.Request) {},
		Transport: rt,
	}
	http.ListenAndServe(*addrFlag, &proxy)
}

type roundTripper struct {
	mu  sync.Mutex
	in  *bufio.Reader
	out *bufio.Writer
}

func newRoundTripper(in io.Reader, out io.Writer) *roundTripper {
	return &roundTripper{
		in:  bufio.NewReader(in),
		out: bufio.NewWriter(out),
	}
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	req.Write(rt.out)
	rt.out.Flush()

	return http.ReadResponse(rt.in, req)
}
