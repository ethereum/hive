package hiveproxy

import (
	"io"
	"io/fs"
	"net"
	"net/http"
	"testing"
)

func TestE2E(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	pc := RunClient(cr, cw, l)
	defer pc.Close()

	var called bool
	test := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	ps := RunServer(sr, sw, test)
	defer ps.Close()

	http.Get("http://" + l.Addr().String())

	if !called {
		t.Fatal("handler not called")
	}
}

func TestSource(t *testing.T) {
	fs.WalkDir(Source, ".", func(path string, d fs.DirEntry, err error) error {
		t.Log(path)
		return err
	})
}
