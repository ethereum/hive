package main

import (
	"flag"
	"net"
	"os"

	"github.com/ethereum/hive/hiveproxy"
)

func main() {
	addrFlag := flag.String("addr", ":8081", "listening address")
	flag.Parse()

	l, err := net.Listen("tcp", *addrFlag)
	if err != nil {
		panic(err)
	}
	hiveproxy.RunFrontend(os.Stdin, os.Stdout, l)
	select {}
}
