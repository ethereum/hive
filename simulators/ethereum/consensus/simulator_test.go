package main

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

type fakeApi struct{}

//func (fakeApi) GetDockerInfo() (*docker.DockerInfo, error) {
//	panic("implement me")
//}

func (fakeApi) GetClientIP(string) (*string, error) {
	panic("implement me")
}

func (fakeApi) GetClientEnode(string) (*string, error) {
	panic("implement me")
}

func (fakeApi) GetClientTypes() ([]string, error) {
	return []string{"fakeclient", "fakeclient2"}, nil
}

func (fakeApi) StartNewNode(map[string]string) (string, net.IP, error) {
	return "123123", net.ParseIP("127.1.1.1s"), nil
}

func (fakeApi) Log(string) error {
	panic("implement me")
}

func (fakeApi) AddResults(success bool, nodeID string, name string, errMsg string, duration time.Duration) error {
	return nil
}

func (fakeApi) KillNode(string) error {
	panic("implement me")
}

type FakeTestExecutor struct {
	id int
}

func (f *FakeTestExecutor) run(testChan chan *testcase) {
	var i = 0
	for t := range testChan {
		fmt.Printf("worker(%d) objects %p %p\n", f.id, t, &(t.blockTest))
		i++
	}
}

func TestDelivery(t *testing.T) {
	//Try to connect to the simulator host and get the client list
	//hivesim := "none"
	//host := &common.SimulatorHost{
	//	HostURI: &hivesim,
	//}
	//availableClients, _ := host.GetClientTypes()
	//log.Info("Got clients", "clients", availableClients)
	fileRoot := "./tests/BlockchainTests/"
	testCh := deliverTests(fileRoot)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			b := FakeTestExecutor{i}
			b.run(testCh)
			wg.Done()
		}()
	}
	wg.Wait()

}
