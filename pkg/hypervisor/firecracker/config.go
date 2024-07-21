package firecracker

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"vistara-node/pkg/errors"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/models"
	"vistara-node/pkg/network"
)

type ConfigOption func(cfg *VmmConfig) error

func CreateConfig(opts ...ConfigOption) (*VmmConfig, error) {
	cfg := &VmmConfig{}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("creating firecracker configuration: %w", err)
		}
	}

	return cfg, nil
}

func WithMicroVM(vm *models.MicroVM, status *models.NetworkInterfaceStatus, vsockPath string) ConfigOption {
	return func(cfg *VmmConfig) error {
		if vm == nil {
			return errors.ErrSpecRequired
		}

		cfg.MachineConfig = MachineConfig{
			MemSizeMib: int64(vm.Spec.MemoryInMb),
			VcpuCount:  int64(vm.Spec.VCPU),
			SMT:        runtime.GOARCH == "amd64",
		}

		cfg.NetDevices = []NetworkInterfaceConfig{
			{
				IfaceID:     "eth0",
				HostDevName: status.HostDeviceName,
				GuestMAC:    vm.Spec.GuestMAC,
			},
		}

		cfg.Mmds = &MMDSConfig{
			Version:           MMDSVersion1,
			NetworkInterfaces: []string{cfg.NetDevices[0].IfaceID},
		}

		cfg.BlockDevices = []BlockDeviceConfig{
			{
				ID:           "rootfs",
				IsReadOnly:   true,
				IsRootDevice: true,
				PathOnHost:   vm.Spec.RootfsPath,
				CacheType:    CacheTypeUnsafe,
			},
			{
				ID:           "image",
				IsReadOnly:   false,
				IsRootDevice: false,
				PathOnHost:   vm.Spec.ImagePath,
				CacheType:    CacheTypeUnsafe,
			},
		}

		cfg.VsockDevice = &VsockDeviceConfig{
			GuestCID: 0,
			UDSPath:  vsockPath,
		}

		tapIdx, err := strconv.Atoi(strings.ReplaceAll(status.HostDeviceName, "hypercore-", ""))
		if err != nil {
			return fmt.Errorf("invalid interface %s: %w", status.HostDeviceName, err)
		}

		tapDetails := network.GetTapDetails(tapIdx)

		kernelCmdLine := DefaultKernelCmdLine()
		kernelCmdLine.Set("ip", fmt.Sprintf("%s::%s:%s::eth0::%s", tapDetails.VMIP.To4(), tapDetails.TapIP.To4(), tapDetails.Mask.To4(), "1.1.1.1"))
		kernelArgs := kernelCmdLine.String()

		bootSourceConfig := BootSourceConfig{
			KernelImagePage: vm.Spec.Kernel,
			BootArgs:        &kernelArgs,
		}
		cfg.BootSource = bootSourceConfig

		return nil
	}
}

// DefaultKernelCmdLine is the default recommended kernel parameter list.
//
// console=ttyS0   [KLN] Output console device and options
// reboot=k        [KNL] reboot_type=kbd
// panic=1         [KNL] Kernel behaviour on panic: delay <timeout>
//
//	timeout > 0: seconds before rebooting
//	timeout = 0: wait forever
//	timeout < 0: reboot immediately
//
// pci=off         [X86] don't probe for the PCI bus
// i8042.noaux     [HW]  Don't check for auxiliary (== mouse) port
// i8042.nomux     [HW]  Don't check presence of an active multiplexing
//
//	controller
//
// i8042.nopnp     [HW]  Don't use ACPIPnP / PnPBIOS to discover KBD/AUX
//
//	controllers
//
// i8042.dumbkbd   [HW]  Pretend that controller can only read data from
//
//	keyboard and cannot control its state
//	(Don't attempt to blink the leds)
//
// Read more:
// https://www.kernel.org/doc/html/v5.15/admin-guide/kernel-parameters.html
func DefaultKernelCmdLine() shared.KernelCmdLine {
	return shared.KernelCmdLine{
		"console":                             "ttyS0",
		"reboot":                              "k",
		"panic":                               "1",
		"pci":                                 "off",
		"i8042.noaux":                         "",
		"i8042.nomux":                         "",
		"i8042.nopnp":                         "",
		"i8042.dumbkbd":                       "",
		"systemd.journald.forward_to_console": "",
		"systemd.unit":                        "firecracker.target",
		"init":                                "/sbin/overlay-init",
	}
}

func WithState(vmState *State) ConfigOption {
	return func(cfg *VmmConfig) error {
		cfg.Logger = &LoggerConfig{
			LogPath:       vmState.LogPath(),
			Level:         LogLevelDebug,
			ShowLevel:     true,
			ShowLogOrigin: true,
		}
		cfg.Metrics = &MetricsConfig{
			Path: vmState.MetricsPath(),
		}

		return nil
	}
}
