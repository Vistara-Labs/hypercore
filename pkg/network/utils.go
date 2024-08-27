package network

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func GetLinkMacIP(linkName string) (net.HardwareAddr, net.IP, error) {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return net.HardwareAddr{}, net.IP{}, fmt.Errorf("failed to get link %s: %w", linkName, err)
	}

	routes, err := netlink.RouteList(link, 4)
	if err != nil {
		return net.HardwareAddr{}, net.IP{}, fmt.Errorf("failed to get routes for link %s: %w", linkName, err)
	}

	if len(routes) == 0 {
		return net.HardwareAddr{}, net.IP{}, fmt.Errorf("got no routes for link %s", linkName)
	}

	return link.Attrs().HardwareAddr, routes[0].Src, nil
}
