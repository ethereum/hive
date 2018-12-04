package main

import (
	"errors"
	"net"
	"strings"

	"gopkg.in/inconshreveable/log15.v2"
)

// lookupBridgeIP attempts to locate the IPv4 address of the local docker0 bridge
// network adapter.
func lookupBridgeIP(logger log15.Logger) (net.IP, error) {
	// Find the local IPv4 address of the docker0 bridge adapter
	interfaes, err := net.Interfaces()
	if err != nil {
		logger.Error("failed to list network interfaces", "error", err)
		return nil, err
	}
	// Iterate over all the interfaces and find the docker0 bridge
	for _, iface := range interfaes {
		if iface.Name == "docker0" || strings.Contains(iface.Name, "vEthernet") {
			// Retrieve all the addresses assigned to the bridge adapter
			addrs, err := iface.Addrs()
			if err != nil {
				logger.Error("failed to list docker bridge addresses", "error", err)
				return nil, err
			}
			// Find a suitable IPv4 address and return it
			for _, addr := range addrs {
				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil {
					logger.Error("failed to list parse address", "address", addr, "error", err)
					return nil, err
				}
				if ipv4 := ip.To4(); ipv4 != nil {
					return ipv4, nil
				}
			}
		}
	}
	// Crap, no IPv4 found, bounce
	return nil, errors.New("not found")
}
