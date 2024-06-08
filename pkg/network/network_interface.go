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
		"step":  s.Name(),
	})
	logger.Debug("running Create to create network interface")

	if s.status == nil {
		return errors.ErrMissingStatusInfo
	}

	ifaceName, err := NewIfaceName()
	if err != nil {
		return fmt.Errorf("creating network interface name: %w", err)
	}

	s.status.HostDeviceName = ifaceName

	deviceName := s.status.HostDeviceName

	exists, err := s.svc.IfaceExists(ctx, deviceName)
	if err != nil {
		return fmt.Errorf("checking if networking interface exists: %w", err)
	}

	if exists {
		details, detailsErr := s.svc.IfaceDetails(ctx, deviceName)
		if detailsErr != nil {
			return fmt.Errorf("getting interface details: %w", detailsErr)
		}

		s.status.HostDeviceName = deviceName
		s.status.Index = details.Index
		s.status.MACAddress = details.MAC

		return nil
	}

	input := &ports.IfaceCreateInput{
		DeviceName: deviceName,
		Type:       s.iface.Type,
		MAC:        s.iface.GuestMAC,
		Attach:     true,
		BridgeName: s.iface.BridgeName,
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

	return nil
}

// Do will perform the operation/procedure.
// func (s *createInterface) Do(ctx context.Context) ([]planner.Procedure, error) {
// 	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
// 		"step":  s.Name(),
// 		"iface": s.iface.GuestDeviceName,
// 	})
// 	logger.Debug("running step to create network interface")

// 	if s.iface.GuestDeviceName == "" {
// 		return nil, errors.ErrGuestDeviceNameRequired
// 	}

// 	if s.status == nil {
// 		return nil, errors.ErrMissingStatusInfo
// 	}

// 	if s.status.HostDeviceName == "" {
// 		ifaceName, err := network.NewIfaceName(s.iface.Type)
// 		if err != nil {
// 			return nil, fmt.Errorf("creating network interface name: %w", err)
// 		}

// 		s.status.HostDeviceName = ifaceName
// 	}

// 	deviceName := s.status.HostDeviceName

// 	exists, err := s.svc.IfaceExists(ctx, deviceName)
// 	if err != nil {
// 		return nil, fmt.Errorf("checking if networking interface exists: %w", err)
// 	}

// 	if exists {
// 		details, detailsErr := s.svc.IfaceDetails(ctx, deviceName)
// 		if detailsErr != nil {
// 			return nil, fmt.Errorf("getting interface details: %w", detailsErr)
// 		}

// 		s.status.HostDeviceName = deviceName
// 		s.status.Index = details.Index
// 		s.status.MACAddress = details.MAC

// 		return nil, nil
// 	}

// 	input := &ports.IfaceCreateInput{
// 		DeviceName: deviceName,
// 		Type:       s.iface.Type,
// 		MAC:        s.iface.GuestMAC,
// 		Attach:     true,
// 		BridgeName: s.iface.BridgeName,
// 	}

// 	if s.iface.Type == models.IfaceTypeTap && s.iface.AllowMetadataRequests {
// 		input.Attach = false
// 	}

// 	output, err := s.svc.IfaceCreate(ctx, *input)
// 	if err != nil {
// 		return nil, fmt.Errorf("creating network interface: %w", err)
// 	}

// 	s.status.HostDeviceName = deviceName
// 	s.status.Index = output.Index
// 	s.status.MACAddress = output.MAC

// 	return nil, nil
// }

// func (s *createInterface) Verify(ctx context.Context) error {
// 	return nil
// }
