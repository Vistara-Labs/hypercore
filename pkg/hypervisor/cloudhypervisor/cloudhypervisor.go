package cloudhypervisor

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
	"vistara-node/pkg/api/types"
	cloudinit "vistara-node/pkg/cloudinit"
	cloudinit_net "vistara-node/pkg/cloudinit/network"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/network"
	"vistara-node/pkg/ports"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

const (
	HypervisorName = "cloudhypervisor"
)

// Config represents the configuration options for the Firecracker infrastructure.
type Config struct {
	// CloudHypervisorBin is the cloud hypervisor binary to use.
	CloudHypervisorBin string
	// StateRoot is the folder to store any required cloud hypervisor state (i.e. socks, pid, log files).
	StateRoot string
	// RunDetached indicates that the cloud hypervisor processes should be run detached (a.k.a daemon) from the parent process.
	RunDetached bool
	// DeleteVMTimeout is the timeout to wait for the microvm to be deleted.
	DeleteVMTimeout time.Duration
}

type CloudHypervisorService struct {
	config *Config

	networkSvc ports.NetworkService
	diskSvc    ports.DiskService
	fs         afero.Fs
}

func New(cfg *Config, networkSvc ports.NetworkService, diskSvc ports.DiskService, fs afero.Fs) ports.MicroVMService {
	return &CloudHypervisorService{
		config:     cfg,
		networkSvc: networkSvc,
		diskSvc:    diskSvc,
		fs:         fs,
	}
}

func (c *CloudHypervisorService) Start(ctx context.Context, vm *models.MicroVM) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "cloudhypervisor_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Debugf("creating microvm inside cloudhypervisor start")

	if vm.Spec.Kernel == "" || vm.Spec.RootfsPath == "" || vm.Spec.HostNetDev == "" || vm.Spec.GuestMAC == "" {
		return errors.New("missing fields from model")
	}

	vmState := NewState(vm.ID, c.config.StateRoot, c.fs)

	if err := c.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	// We will have only one interface, i.e. the TAP device
	status := &models.NetworkInterfaceStatus{}
	nface := network.NewNetworkInterface(&vm.ID, &models.NetworkInterface{
		GuestMAC:   vm.Spec.GuestMAC,
		Type:       models.IfaceTypeTap,
		BridgeName: vm.Spec.HostNetDev,
	}, status, c.networkSvc)
	if err := nface.Create(ctx); err != nil {
		return fmt.Errorf("creating network interface %w", err)
	}

	if err := c.createCloudInitImage(ctx, vm, vmState, status); err != nil {
		return fmt.Errorf("creating metadata image: %w", err)
	}

	proc, err := c.startMicroVM(vm, vmState, status, c.config.RunDetached)

	if err != nil {
		return fmt.Errorf("starting cloudhypervisor process %w", err)
	}

	if err = vmState.SetPid(proc.Pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", proc.Pid, err)
	}

	if err = vmState.SetRuntimeState(RuntimeState{HostIface: status.HostDeviceName}); err != nil {
		return fmt.Errorf("saving runtime state: %w", err)
	}

	return nil
}

func (c *CloudHypervisorService) createCloudInitImage(ctx context.Context, vm *models.MicroVM, vmState State, networkInterfaceStatus *models.NetworkInterfaceStatus) error {
	cfg := &cloudinit_net.Network{
		Version: 2,
		Ethernet: map[string]cloudinit_net.Ethernet{
			"ens3": {
				GatewayIPv4: networkInterfaceStatus.TapDetails.VmIp.To4().String(),
				Addresses:   []string{networkInterfaceStatus.TapDetails.VmIp.To4().String() + "/30"},
			},
		},
	}

	yamlNetworkCfg, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling network data: %w", err)
	}

	input := ports.DiskCreateInput{
		Path:       vmState.CloudInitImage(),
		Size:       "8Mb",
		VolumeName: cloudinit.VolumeName,
		Type:       ports.DiskTypeFat32,
		Overwrite:  true,
		Files: []ports.DiskFile{
			{
				Path:          "/network-config",
				ContentBase64: base64.StdEncoding.EncodeToString(yamlNetworkCfg),
			},
			// TODO remove hardcoded user config
			{
				Path:          "/user-data",
				ContentBase64: "I2Nsb3VkLWNvbmZpZwp1c2VyczoKICAtIG5hbWU6IGNsb3VkCiAgICBwYXNzd2Q6ICQ2JDcxMjU3ODc3NTFhOGQxOGEkc0h3R3lTb21VQTFQYXdpTkZXVkNLWVFOLkVjLld6ejBKdFBQTDFNdnpGcmt3bW9wMmRxNy40Q1lmMDNBNW9lbVBRNHBPRkNDcnRDZWx2RkJFbGUvSy4KICAgIHN1ZG86IEFMTD0oQUxMKSBOT1BBU1NXRDpBTEwKICAgIGxvY2tfcGFzc3dkOiBGYWxzZQogICAgaW5hY3RpdmU6IEZhbHNlCiAgICBzaGVsbDogL2Jpbi9iYXNoCgpzc2hfcHdhdXRoOiBUcnVlCg==",
			},
			{
				Path:          "/meta-data",
				ContentBase64: "aW5zdGFuY2UtaWQ6IGNsb3VkCmxvY2FsLWhvc3RuYW1lOiBjbG91ZAo=",
			},
		},
	}

	if err := c.diskSvc.Create(ctx, input); err != nil {
		return fmt.Errorf("creating cloud-init volume: %w", err)
	}

	return nil
}

func (c *CloudHypervisorService) startMicroVM(vm *models.MicroVM, vmState State, networkInterfaceStatus *models.NetworkInterfaceStatus, detached bool) (*os.Process, error) {
	args := []string{
		"--api-socket",
		vmState.SockPath(),
		"--log-file",
		vmState.LogPath(),
		"-v",
		"--serial", "tty",
		"--console", "off",
		"--kernel", vm.Spec.Kernel,
		"--cpus", fmt.Sprintf("boot=%d", vm.Spec.VCPU),
		"--memory", fmt.Sprintf("size=%dM", vm.Spec.MemoryInMb),
		"--disk", fmt.Sprintf("path=%s", vm.Spec.RootfsPath), fmt.Sprintf("path=%s,readonly=on", vmState.CloudInitImage()),
		"--net", fmt.Sprintf("tap=%s,mac=%s", networkInterfaceStatus.HostDeviceName, networkInterfaceStatus.MACAddress),
	}

	fmt.Println(args)

	stdOutFile, err := c.fs.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
	}

	stdErrFile, err := c.fs.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening sterr file %s: %w", vmState.StderrPath(), err)
	}

	cmd := exec.Command(c.config.CloudHypervisorBin, args...)

	cmd.Stderr = stdErrFile
	cmd.Stdout = stdOutFile
	cmd.Stdin = &bytes.Buffer{}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting cloudhypervisor process: %w", err)
	}

	// Reap the process
	go func() { cmd.Wait() }()

	return cmd.Process, nil
}

func (c *CloudHypervisorService) ensureState(vmState State) error {
	if err := c.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
		return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
	}

	logFile, err := c.fs.OpenFile(vmState.LogPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening log file %s: %w", vmState.LogPath(), err)
	}

	defer logFile.Close()

	return nil
}

func (c *CloudHypervisorService) GetRuntimeData(ctx context.Context, vm *models.MicroVM) (*types.MicroVMRuntimeData, error) {
	vmState := NewState(vm.ID, c.config.StateRoot, c.fs)

	state, err := vmState.RuntimeState()
	if err != nil {
		return nil, err
	}

	return &types.MicroVMRuntimeData{
		NetworkInterface: state.HostIface,
	}, nil
}

func (c *CloudHypervisorService) Stop(ctx context.Context, vm *models.MicroVM) error {
	vmState := NewState(vm.ID, c.config.StateRoot, c.fs)

	pid, err := vmState.PID()
	if err != nil {
		return fmt.Errorf("failed to get PID: %w", err)
	}

	// TODO exec the reboot command from the guest via ssh to perform a clean
	// shutdown, and send SIGTERM, timeout, SIGKILL
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process with pid %d: %w", pid, err)
	}

	if err = proc.Kill(); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	state, err := vmState.RuntimeState()
	if err != nil {
		return fmt.Errorf("failed to get runtime state: %w", err)
	}

	if err = c.networkSvc.IfaceDelete(context.Background(), ports.DeleteIfaceInput{
		DeviceName: state.HostIface,
	}); err != nil {
		return fmt.Errorf("failed to delete network interface %s: %w", state.HostIface, err)
	}

	if err = vmState.Delete(); err != nil {
		return fmt.Errorf("failed to delete vmState dir: %w", err)
	}

	return nil
}

func (c *CloudHypervisorService) Pid(ctx context.Context, vm *models.MicroVM) (int, error) {
	panic("TODO")
}

func (c *CloudHypervisorService) VSockPath(vm *models.MicroVM) string {
	panic("Unsupported")
}

func (c *CloudHypervisorService) State(ctx context.Context, id string) (ports.MicroVMState, error) {
	// Implement Firecracker status check logic
	return ports.MicroVMStateRunning, nil
}

func (c *CloudHypervisorService) Metrics(ctx context.Context, id models.VMID) (ports.MachineMetrics, error) {
	// Implement Firecracker status check logic
	machineMetrics := shared.MachineMetrics{}
	return machineMetrics, nil
}
