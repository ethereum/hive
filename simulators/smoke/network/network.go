package main

import (
	"fmt"
	"net"

	"github.com/ethereum/hive/hivesim"
)

func main() {
	suite := hivesim.Suite{
		Name:        "network",
		Description: "This suite tests the simulation API endpoints related to docker networks.",
	}
	suite.Add(hivesim.TestSpec{
		Name: "connection on network1",
		Description: `In this test, the client is created, then added to a new docker network.
The test then tries to connect to the client container via TCP on the new network.`,
		Run: iptest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func iptest(t *hivesim.T) {
	clientDef, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatal(err)
	}
	if len(clientDef) == 0 {
		t.Fatal("no clients available")
	}
	client := t.StartClient(clientDef[0].Name)

	// This creates a network and connects both the client and the simulation container to it.
	network := "network1"
	err = t.Sim.CreateNetwork(t.SuiteID, network)
	if err != nil {
		t.Fatal("can't not create network:", err)
	}
	if err := t.Sim.ConnectContainer(t.SuiteID, network, client.Container); err != nil {
		t.Fatal("can't connect container to network:", err)
	}
	if err := t.Sim.ConnectContainer(t.SuiteID, network, "simulation"); err != nil {
		t.Fatal("can't connect simulation container to network:", err)
	}

	// Now get the IP of the client and connect to it via TCP.
	clientIP, err := t.Sim.ContainerNetworkIP(t.SuiteID, network, client.Container)
	if err != nil {
		t.Fatal("can't get IP address of container:", err)
	}
	t.Log("client IP", clientIP)
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", clientIP, 8545))
	if err != nil {
		t.Fatal("can't dial client:", err)
	}
	conn.Close()

	// Make sure ContainerNetworkIP works with the simulation container as well.
	simIP, err := t.Sim.ContainerNetworkIP(t.SuiteID, network, "simulation")
	if err != nil {
		t.Fatal("can't get IP address of container:", err)
	}
	t.Log("simulation container IP", simIP)

	// Make sure the IP address of the client container on the bridge network matches
	// what is returned by StartClient
	clientBridgeIP, err := t.Sim.ContainerNetworkIP(t.SuiteID, "bridge", client.Container)
	if err != nil {
		t.Fatal("can't get IP address of container:", err)
	}
	if clientBridgeIP != client.IP.String() {
		t.Fatal("ip address mismatch", "expected", client.IP.String(), "got", clientBridgeIP)
	}

	// Disconnect client and simulation from network1.
	if err := t.Sim.DisconnectContainer(t.SuiteID, network, client.Container); err != nil {
		t.Fatal("can't disconnect client from network:", err)
	}
	if err := t.Sim.DisconnectContainer(t.SuiteID, network, "simulation"); err != nil {
		t.Fatal("can't disconnect simulation from network:", err)
	}
	// Remove network1.
	if err := t.Sim.RemoveNetwork(t.SuiteID, network); err != nil {
		t.Fatal("can't remove network:", err)
	}
}
