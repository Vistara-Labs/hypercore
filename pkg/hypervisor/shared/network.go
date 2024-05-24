package shared

import (
	"encoding/base64"
	"fmt"
	"vistara-node/pkg/cloudinit/network"
	"vistara-node/pkg/errors"
	"vistara-node/pkg/models"

	"gopkg.in/yaml.v2"
)

const (
	cloudInitNetVersion = 2
)

// Bool will return a pointer value of the given parameter.
func Bool(b bool) *bool {
	return &b
}

// String will return a pointer value of the given parameter.
func String(str string) *string {
	return &str
}

// GenerateNetworkConfig will generate the cloud-init network config from the vm spec.
func GenerateNetworkConfig(vm *models.MicroVM) (string, error) {
	netConf := &network.Network{
		Version:  cloudInitNetVersion,
		Ethernet: map[string]network.Ethernet{},
	}

	for i := range vm.Spec.NetworkInterfaces {
		iface := vm.Spec.NetworkInterfaces[i]

		status, ok := vm.Status.NetworkInterfaces[iface.GuestDeviceName]

		if !ok {
			return "", errors.NewNetworkInterfaceStatusMissing(iface.GuestDeviceName)
		}

		macAddress := getMacAddress(&iface, status)

		eth := &network.Ethernet{
			Match:          network.Match{},
			DHCP4:          Bool(true),
			DHCP6:          Bool(true),
			DHCPIdentifier: String(network.DhcpIdentifierMac),
		}

		if macAddress != "" {
			eth.Match.MACAddress = macAddress
		} else {
			eth.Match.Name = iface.GuestDeviceName
		}

		if iface.StaticAddress != nil {
			if err := configureStaticEthernet(&iface, eth); err != nil {
				fmt.Printf("\nconfiguring static ethernet err is : %v\n", err)

				return "", fmt.Errorf("configuring static ethernet address: %w", err)
			}
		}

		netConf.Ethernet[iface.GuestDeviceName] = *eth
	}

	nd, err := yaml.Marshal(netConf)

	if err != nil {
		return "", fmt.Errorf("marshalling network data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(nd), nil
}

func configureStaticEthernet(iface *models.NetworkInterface, eth *network.Ethernet) error {
	eth.Addresses = []string{string(iface.StaticAddress.Address)}

	if iface.StaticAddress.Gateway != nil {
		isIPv4, err := iface.StaticAddress.Gateway.IsIPv4()
		if err != nil {
			return fmt.Errorf("parsing gateway address: %w", err)
		}

		ipAddr, err := iface.StaticAddress.Gateway.IP()
		if err != nil {
			return fmt.Errorf("parsing gateway address: %w", err)
		}

		if isIPv4 {
			eth.GatewayIPv4 = ipAddr
		} else {
			eth.GatewayIPv6 = ipAddr
		}
	}

	if len(iface.StaticAddress.Nameservers) > 0 {
		eth.Nameservers = network.Nameservers{
			Addresses: []string{},
		}

		for nsIndex := range iface.StaticAddress.Nameservers {
			ns := iface.StaticAddress.Nameservers[nsIndex]
			eth.Nameservers.Addresses = append(eth.Nameservers.Addresses, ns)
		}
	}

	eth.DHCP4 = Bool(false)
	eth.DHCP6 = Bool(false)

	return nil
}

func getMacAddress(iface *models.NetworkInterface, status *models.NetworkInterfaceStatus) string {
	if iface.Type == models.IfaceTypeMacvtap {
		return status.MACAddress
	}

	return iface.GuestMAC
}
