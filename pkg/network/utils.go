package network

import (
	"fmt"
	"net"
)

func GetLinkMacIP(linkName string) (net.HardwareAddr, net.IP, error) {
	link, err := net.InterfaceByName(linkName)
	if err != nil {
		return net.HardwareAddr{}, net.IP{}, fmt.Errorf("failed to get link %s: %w", linkName, err)
	}

	addrs, err := link.Addrs()
	if err != nil {
		return net.HardwareAddr{}, net.IP{}, fmt.Errorf("failed to get addresses for link %s: %w", linkName, err)
	}

	for _, addr := range addrs {
		ip, ok := addr.(*net.IPNet)
		if ok {
			return link.HardwareAddr, ip.IP, nil
		}
	}

	return net.HardwareAddr{}, net.IP{}, fmt.Errorf("no ip addresses found for link %s", linkName)
}
