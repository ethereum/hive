package main

import (
	crand "crypto/rand"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/ethereum/hive/hivesim"
)

type dualStackNetwork struct {
	name       string
	ipv4Subnet string
	ipv6Subnet string
	simIPv4    string
	simIPv6    string
	clientIPv4 string
	clientIPv6 string
}

func runDiscv5DualStackDockerNetworkTests(t *hivesim.T) {
	clients, err := t.Sim.ClientsWithRole("eth1")
	if err != nil {
		t.Fatal("can't list eth1 clients:", err)
	}
	if len(clients) == 0 {
		t.Fatal("no eth1 clients available")
	}

	for _, client := range clients {
		client := client
		t.Run(hivesim.TestSpec{
			Name:      fmt.Sprintf("self ENR is reachable over IPv6 with configured IPv4 and IPv6 endpoints (%s)", client.Name),
			AlwaysRun: true,
			Run: func(t *hivesim.T) {
				runDiscv5DualStackDockerNetworkTest(t, client)
			},
		})
	}
}

func runDiscv5DualStackDockerNetworkTest(t *hivesim.T, clientDef *hivesim.ClientDefinition) {
	network := createDualStackNetwork(t, clientDef.Name)
	params := eth1Discv5Params().
		Set("HIVE_LOCAL_IP", "::").
		Set("HIVE_EXTERNAL_IP", network.clientIPv4).
		Set("HIVE_EXTERNAL_IP_V4", network.clientIPv4).
		Set("HIVE_EXTERNAL_IP_V6", network.clientIPv6)
	client := t.StartClient(clientDef.Name,
		params,
		hivesim.WithInitialNetworkConfig(network.name, hivesim.NetworkEndpointConfig{
			IPv4Address: network.clientIPv4,
			IPv6Address: network.clientIPv6,
		}),
	)
	requireContainerNetworkAddress(t, network.name, client.Container, network.clientIPv4, network.clientIPv6)

	nodeURLv4, err := client.EnodeURLNetwork(network.name)
	if err != nil {
		t.Fatal("can't get client IPv4 enode URL:", err)
	}
	runDiscv5ExpectedEndpointTest(t, client.Type+" IPv4", nodeURLv4, network.simIPv4, network.clientIPv4, network.clientIPv6)

	nodeURLv6, err := client.EnodeURLNetworkIPv6(network.name)
	if err != nil {
		t.Fatal("can't get client IPv6 enode URL:", err)
	}
	runDiscv5ExpectedEndpointTest(t, client.Type+" IPv6", nodeURLv6, network.simIPv6, network.clientIPv4, network.clientIPv6)
}

func runDiscv5ExpectedEndpointTest(t *hivesim.T, clientName, nodeURL, listenIP, expectedIPv4, expectedIPv6 string) {
	cmd := exec.Command(
		"./devp2p", "discv5", "test",
		"--run", "FindnodeZeroDistance",
		"--tap",
		"--listen1", listenIP,
		"--listen2", listenIP,
		"--expect-ip", expectedIPv4,
		"--expect-ip6", expectedIPv6,
		nodeURL,
	)
	if err := runTAP(t, clientName, cmd); err != nil {
		t.Fatal(err)
	}
}

func createDualStackNetwork(t *hivesim.T, clientName string) dualStackNetwork {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		network := newDualStackNetwork(t, clientName)
		err := t.Sim.CreateNetworkWithConfig(t.SuiteID, network.name, hivesim.NetworkConfig{
			IPv4Subnet: network.ipv4Subnet,
			IPv6Subnet: network.ipv6Subnet,
		})
		if err != nil {
			lastErr = err
			continue
		}
		if err := t.Sim.ConnectContainerWithConfig(t.SuiteID, network.name, "simulation", hivesim.NetworkEndpointConfig{
			IPv4Address: network.simIPv4,
			IPv6Address: network.simIPv6,
		}); err != nil {
			t.Fatal("can't connect simulation to dual-stack network:", err)
		}
		requireContainerNetworkAddress(t, network.name, "simulation", network.simIPv4, network.simIPv6)
		return network
	}
	t.Fatalf("can't create dual-stack network after retries: %v", lastErr)
	return dualStackNetwork{}
}

func newDualStackNetwork(t *hivesim.T, clientName string) dualStackNetwork {
	random := make([]byte, 6)
	if _, err := crand.Read(random); err != nil {
		t.Fatal("can't generate random Docker subnet:", err)
	}
	ipv4Subnet := fmt.Sprintf("10.%d.%d.0/24", 64+int(random[0])%64, int(random[1]))
	ipv6Subnet := fmt.Sprintf("fd00:%02x%02x:%02x%02x:%02x%02x::/64", random[2], random[3], random[4], random[5], random[0], random[1])
	return dualStackNetwork{
		name:       fmt.Sprintf("dualstack-%s-%d", safeNetworkSuffix(clientName), t.TestID),
		ipv4Subnet: ipv4Subnet,
		ipv6Subnet: ipv6Subnet,
		simIPv4:    subnetHost(t, ipv4Subnet, 2),
		simIPv6:    subnetHost(t, ipv6Subnet, 2),
		clientIPv4: subnetHost(t, ipv4Subnet, 3),
		clientIPv6: subnetHost(t, ipv6Subnet, 3),
	}
}

func subnetHost(t *hivesim.T, cidr string, host byte) string {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("invalid test subnet %q: %v", cidr, err)
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ipv4 = append(net.IP(nil), ipv4...)
		ipv4[3] = host
		return ipv4.String()
	}
	ipv6 := ip.To16()
	if ipv6 == nil {
		t.Fatalf("invalid test subnet %q", cidr)
	}
	ipv6 = append(net.IP(nil), ipv6...)
	ipv6[15] = host
	return ipv6.String()
}

func safeNetworkSuffix(value string) string {
	suffix := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, value)
	return strings.Trim(suffix, "-")
}

func requireContainerNetworkAddress(t *hivesim.T, networkName string, containerID string, wantIPv4 string, wantIPv6 string) {
	ipv4, err := t.Sim.ContainerNetworkIP(t.SuiteID, networkName, containerID)
	if err != nil {
		t.Fatal("can't get container IPv4 address:", err)
	}
	if ipv4 != wantIPv4 {
		t.Fatalf("container IPv4 on %s got %s, want %s", networkName, ipv4, wantIPv4)
	}
	ipv6, err := t.Sim.ContainerNetworkIPv6(t.SuiteID, networkName, containerID)
	if err != nil {
		t.Fatal("can't get container IPv6 address:", err)
	}
	if ipv6 != wantIPv6 {
		t.Fatalf("container IPv6 on %s got %s, want %s", networkName, ipv6, wantIPv6)
	}
}
