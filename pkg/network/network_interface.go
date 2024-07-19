package network

import (
	"context"
	"fmt"
	"vistara-node/pkg/errors"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"

	"github.com/sirupsen/logrus"
)

func NewNetworkInterface(vmid *models.VMID,
	iface *models.NetworkInterface,
	status *models.NetworkInterfaceStatus,
	svc ports.NetworkService,
) *createInterface {
	return &createInterface{
		vmid:   vmid,
		iface:  iface,
		svc:    svc,
		status: status,
	}
}

type createInterface struct {
	vmid   *models.VMID
	iface  *models.NetworkInterface
	status *models.NetworkInterfaceStatus

	svc ports.NetworkService
}

// Create network interface
func (s *createInterface) Name() string {
	return "create_network_interface"
}

// Create will create the network interface.
func (s *createInterface) Create(ctx context.Context) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("running Create to create network interface")

	if s.status == nil {
		return errors.ErrMissingStatusInfo
	}

	ifaceIdx, err := NewIfaceIdx()
	if err != nil {
		return fmt.Errorf("creating network interface id: %w", err)
	}

	deviceName := fmt.Sprintf("hypercore-%d", ifaceIdx)

	exists, err := s.svc.IfaceExists(ctx, deviceName)
	if err != nil {
		return fmt.Errorf("checking if networking interface exists: %w", err)
	}

	if exists {
		return fmt.Errorf("interface %s already exists", deviceName)
	}

	tapDetails := GetTapDetails(ifaceIdx)

	input := &ports.IfaceCreateInput{
		DeviceName: deviceName,
		Type:       s.iface.Type,
		MAC:        s.iface.GuestMAC,
		Attach:     true,
		BridgeName: s.iface.BridgeName,
		IP4:        tapDetails.TapIp.String() + "/30",
	}

	if s.iface.Type == models.IfaceTypeTap && s.iface.AllowMetadataRequests {
		input.Attach = false
	}

	output, err := s.svc.IfaceCreate(ctx, *input)
	if err != nil {
		return fmt.Errorf("creating network interface: %w", err)
	}

	s.status.HostDeviceName = deviceName
	s.status.Index = output.Index
	s.status.MACAddress = output.MAC
	s.status.TapDetails = tapDetails

	return nil
}
