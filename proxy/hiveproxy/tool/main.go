package main

import (
	"flag"
	"net"
	"os"
)

func main() {
	addrFlag := flag.String("addr", ":8081", "listening address")
	flag.Parse()

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	p := hiveproxy.NewClient(os.Stdin, os.Stdout, l)
	select {}
}
