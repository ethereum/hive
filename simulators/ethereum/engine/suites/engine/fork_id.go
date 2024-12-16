package suite_engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/devp2p"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

type ForkIDSpec struct {
	test.BaseSpec
	ProduceBlocksBeforePeering int
}

func (s ForkIDSpec) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (ft ForkIDSpec) GetName() string {
	var name []string
	name = append(name, fmt.Sprintf("Fork ID: Genesis=%d, %s=%d", ft.GetGenesisTimestamp(), ft.MainFork, ft.ForkTime))
	if ft.PreviousForkTime != 0 {
		name = append(name, fmt.Sprintf("%s=%d", ft.MainFork.PreviousFork(), ft.PreviousForkTime))
	}
	if ft.ProduceBlocksBeforePeering > 0 {
		name = append(name, fmt.Sprintf("BlocksBeforePeering=%d", ft.ProduceBlocksBeforePeering))
	}
	return strings.Join(name, ", ")
}

func (s ForkIDSpec) GetForkConfig() *config.ForkConfig {
	forkConfig := s.BaseSpec.GetForkConfig()
	if forkConfig == nil {
		return nil
	}
	return forkConfig
}

func (ft ForkIDSpec) Execute(t *test.Env) {
	// Produce blocks before starting the test if required
	t.CLMock.ProduceBlocks(ft.ProduceBlocksBeforePeering, clmock.BlockProcessCallbacks{})

	// Get client index's enode
	engine := t.Engine
	conn, err := devp2p.PeerEngineClient(engine, t.CLMock)
	if err != nil {
		t.Fatalf("FAIL: Error peering engine client: %v", err)
	}
	defer conn.Close()
	t.Logf("INFO: Connected to client, remote public key: %s", conn.RemoteKey())

	// Sleep
	time.Sleep(1 * time.Second)

	// Timeout value for all requests
	timeout := 20 * time.Second

	// Send a ping request to verify that we are not immediately disconnected
	pingReq := &devp2p.Ping{}
	if size, err := conn.Write(pingReq); err != nil {
		t.Fatalf("FAIL: Could not write to connection: %v", err)
	} else {
		t.Logf("INFO: Wrote %d bytes to conn", size)
	}

	// Finally wait for the pong response
	msg, err := conn.WaitForResponse(timeout, 0)
	if err != nil {
		t.Fatalf("FAIL: Error waiting for response: %v", err)
	}
	switch msg := msg.(type) {
	case *devp2p.Pong:
		t.Logf("INFO: Received pong response: %v", msg)
	default:
		t.Fatalf("FAIL: Unexpected message type: %v", err)
	}

}
