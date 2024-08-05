package network

import (
	"fmt"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"
)

func NewNetworkInterface(
	iface *models.NetworkInterface,
	status *models.NetworkInterfaceStatus,
) *CreateInterface {
	return &CreateInterface{
		iface:  iface,
		status: status,
	}
}

type CreateInterface struct {
	iface  *models.NetworkInterface
	status *models.NetworkInterfaceStatus
}

// Create network interface
func (s *CreateInterface) Name() string {
	return "create_network_interface"
}

// Create will create the network interface.
func (s *CreateInterface) Create() error {
	ifaceIdx, err := NewIfaceIdx()
	if err != nil {
		return fmt.Errorf("creating network interface id: %w", err)
	}

	deviceName := fmt.Sprintf("hypercore-%d", ifaceIdx)

	exists, err := IfaceExists(deviceName)
	if err != nil {
		return fmt.Errorf("checking if networking interface exists: %w", err)
	}

	if exists {
		return fmt.Errorf("interface %s already exists", deviceName)
	}

	tapDetails := GetTapDetails(ifaceIdx)

	input := &ports.IfaceCreateInput{
		DeviceName: deviceName,
		BridgeName: s.iface.BridgeName,
		IP4:        tapDetails.TapIP.String() + "/30",
	}

	output, err := IfaceCreate(*input)
	if err != nil {
		return fmt.Errorf("creating network interface: %w", err)
	}

	s.status.HostDeviceName = deviceName
	s.status.MACAddress = output.MAC
	s.status.TapDetails = tapDetails

	return nil
}
