package devp2p

//many thanks to dimalinux for the gopackets inspiration
//example code https://github.com/dimalinux/spoofsourceip/blob/master/udpspoof/udp.go

import (
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type udpFrameOptions struct {
	sourceIP, destIP     net.IP
	sourcePort, destPort uint16
	sourceMac, destMac   net.HardwareAddr //we won't implemenent ARP as docker will supply the mac addresses we need
	isIPv6               bool
	payloadBytes         []byte
}

// createSerializedUDPFrame creates an Ethernet frame encapsulating our UDP
// packet for injection to the local network
func createSerializedUDPFrame(opts udpFrameOptions) ([]byte, error) {

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	ethernetType := layers.EthernetTypeIPv4
	if opts.isIPv6 {
		ethernetType = layers.EthernetTypeIPv6
	}
	eth := &layers.Ethernet{
		SrcMAC:       opts.sourceMac,
		DstMAC:       opts.destMac,
		EthernetType: ethernetType,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(opts.sourcePort),
		DstPort: layers.UDPPort(opts.destPort),
		// we configured "Length" and "Checksum" to be set for us
	}
	if !opts.isIPv6 {
		ip := &layers.IPv4{
			SrcIP:    opts.sourceIP,
			DstIP:    opts.destIP,
			Protocol: layers.IPProtocolUDP,
			Version:  4,
			TTL:      32,
		}
		udp.SetNetworkLayerForChecksum(ip)
		err := gopacket.SerializeLayers(buf, serializeOpts, eth, ip, udp, gopacket.Payload(opts.payloadBytes))
		if err != nil {
			return nil, err
		}
	} else {
		ip := &layers.IPv6{
			SrcIP:      opts.sourceIP,
			DstIP:      opts.destIP,
			NextHeader: layers.IPProtocolUDP,
			Version:    6,
			HopLimit:   32,
		}
		ip.LayerType()
		udp.SetNetworkLayerForChecksum(ip)
		err := gopacket.SerializeLayers(buf, serializeOpts, eth, ip, udp, gopacket.Payload(opts.payloadBytes))
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}
