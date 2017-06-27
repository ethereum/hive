package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/whisper/shhclient"
)

var (
	rpcTimeout = 30 * time.Second
)

type TestClient struct {
	*shhclient.Client
	rc *rpc.Client
}

// CallContext is a helper method that forwards a raw RPC request to
// the underlying RPC client. This can be used to call RPC methods
// that are not supported by the ethclient.Client.
func (c *TestClient) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return c.rc.CallContext(ctx, result, method, args...)
}

func createWebsocketClients(hosts []string) []*TestClient {
	if len(hosts) < 3 {
		panic("Supply at least 3 hosts")
	}

	clients := make([]*TestClient, 0, len(hosts))
	for i := 0; i < len(hosts); i++ {
		client, err := rpc.Dial(fmt.Sprintf("ws://%s:8546", hosts[i]))
		if err != nil {
			panic(err)
		}
		clients = append(clients, &TestClient{shhclient.NewClient(client), client})
	}
	return clients
}

func createHTTPClients(hosts []string) []*TestClient {
	if len(hosts) < 3 {
		panic("Supply at least 3 hosts")
	}

	clients := make([]*TestClient, 0, len(hosts))
	for i := 0; i < len(hosts); i++ {
		client, err := rpc.Dial(fmt.Sprintf("http://%s:8545", hosts[i]))
		if err != nil {
			panic(err)
		}
		clients = append(clients, &TestClient{shhclient.NewClient(client), client})
	}
	return clients
}

// runTests is a utility function that calls the unit test with the
// clients as second argument.
func runTest(test func(t *testing.T, clients []*TestClient), clients []*TestClient) func(t *testing.T) {
	return func(t *testing.T) {
		test(t, clients)
	}
}
