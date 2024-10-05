package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func MaskToString(mask net.IPMask) string {
	builder := strings.Builder{}
	for idx, octet := range mask {
		if idx > 0 {
			builder.WriteString(".")
		}
		builder.WriteString(strconv.Itoa(int(octet)))
	}

	return builder.String()
}

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
		if ok && ip.IP.To4() != nil {
			return link.HardwareAddr, ip.IP.To4(), nil
		}
	}

	return net.HardwareAddr{}, net.IP{}, fmt.Errorf("no ip addresses found for link %s", linkName)
}
