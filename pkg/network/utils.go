package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"vistara-node/pkg/models"

	"errors"
	"vistara-node/pkg/ports"

	"github.com/vishvananda/netlink"

	"github.com/coreos/go-iptables/iptables"
	sysctl "github.com/lorenzosaino/go-sysctl"
)

func GetTapDetails(index int) models.TapDetails {
	return models.TapDetails{
		VMIP:  net.IPv4(169, 254, byte(((4*index)+1)/256), byte(((4*index)+1)%256)),
		TapIP: net.IPv4(169, 254, byte(((4*index)+2)/256), byte(((4*index)+2)%256)),
		Mask:  net.IPv4(255, 255, 255, 252),
	}
}

func GetLinkIP(linkName string) (net.IP, error) {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return net.IP{}, fmt.Errorf("failed to get link %s: %w", linkName, err)
	}

	routes, err := netlink.RouteList(link, 4)
	if err != nil {
		return net.IP{}, fmt.Errorf("failed to get routes for link %s: %w", linkName, err)
	}

	if len(routes) == 0 {
		return net.IP{}, fmt.Errorf("got no routes for link %s", linkName)
	}

	return routes[0].Src, nil
}

func NewIfaceIdx() (int, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return 0, fmt.Errorf("failed to enumerate links: %w", err)
	}

	highestLink := -1

	// Get the next highest link available
	for _, link := range links {
		if strings.HasPrefix(link.Attrs().Name, "hypercore-") {
			idxStr := strings.ReplaceAll(link.Attrs().Name, "hypercore-", "")
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return 0, fmt.Errorf("got invalid link %s: %w", link.Attrs().Name, err)
			}

			if idx > highestLink {
				highestLink = idx
			}
		}
	}

	return highestLink + 1, nil
}

func IfaceCreate(input ports.IfaceCreateInput) (*ports.IfaceDetails, error) {
	link := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: input.DeviceName,
		},
		Mode: netlink.TuntapMode(netlink.TCA_CSUM_UPDATE_FLAG_ICMP),
	}

	if err := netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("creating interface %s using netlink: %w", link.Attrs().Name, err)
	}

	macIf, err := netlink.LinkByName(link.Attrs().Name)
	if err != nil {
		return nil, fmt.Errorf("getting interface %s using netlink: %w", link.Attrs().Name, err)
	}

	if err := netlink.LinkSetUp(macIf); err != nil {
		return nil, fmt.Errorf("enabling device %s: %w", macIf.Attrs().Name, err)
	}

	addr, err := netlink.ParseAddr(input.IP4)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TAP IP %s: %w", input.IP4, err)
	}

	if err = netlink.AddrAdd(link, addr); err != nil {
		return nil, fmt.Errorf("failed to add address to TAP device: %w", err)
	}

	err = sysctl.Set("net.ipv4.ip_forward", "1")
	if err != nil {
		return nil, fmt.Errorf("failed to enable IPv4 forwarding: %w", err)
	}

	err = sysctl.Set(fmt.Sprintf("net.ipv4.conf.%s.proxy_arp", link.Attrs().Name), "1")
	if err != nil {
		return nil, fmt.Errorf("failed to enable proxy_arp: %w", err)
	}

	err = sysctl.Set(fmt.Sprintf("net.ipv6.conf.%s.disable_ipv6", link.Attrs().Name), "1")
	if err != nil {
		return nil, fmt.Errorf("failed to enable disable_ipv6: %w", err)
	}

	ipt, err := iptables.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create IPTables instance")
	}

	err = ipt.AppendUnique("nat", "POSTROUTING", "-o", input.BridgeName, "-j", "MASQUERADE")
	if err != nil {
		return nil, fmt.Errorf("failed to add MASQUERADE rule: %w", err)
	}

	err = ipt.InsertUnique("filter", "FORWARD", 1, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	if err != nil {
		return nil, fmt.Errorf("failed to add ACCEPT rule: %w", err)
	}

	err = ipt.InsertUnique("filter", "FORWARD", 1, "-i", link.Attrs().Name, "-o", input.BridgeName, "-j", "ACCEPT")
	if err != nil {
		return nil, fmt.Errorf("failed to add forwarding from parent device %s to TAP device %s: %w", input.BridgeName, link.Attrs().Name, err)
	}

	return &ports.IfaceDetails{
		DeviceName: input.DeviceName,
		MAC:        strings.ToUpper(macIf.Attrs().HardwareAddr.String()),
		Index:      macIf.Attrs().Index,
	}, nil
}

func IfaceDelete(input ports.DeleteIfaceInput) error {
	link, err := netlink.LinkByName(input.DeviceName)
	if err != nil {
		if errors.Is(err, netlink.LinkNotFoundError{}) {
			return fmt.Errorf("failed to lookup network interface %s: %w", input.DeviceName, err)
		}

		return nil
	}

	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("deleting interface %s: %w", link.Attrs().Name, err)
	}

	return nil
}

func IfaceExists(name string) (bool, error) {
	found, _, err := getIface(name)
	if err != nil {
		return false, fmt.Errorf("getting interface %s: %w", name, err)
	}

	return found, nil
}

func getIface(name string) (bool, netlink.Link, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		if errors.Is(err, netlink.LinkNotFoundError{}) {
			return false, nil, fmt.Errorf("failed to lookup network interface %s: %w", name, err)
		}

		return false, nil, nil
	}

	return true, link, nil
}
