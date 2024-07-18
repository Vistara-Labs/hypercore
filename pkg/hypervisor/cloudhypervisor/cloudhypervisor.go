package cloudhypervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"os"
	"os/exec"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/network"
	"vistara-node/pkg/ports"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
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
}

type CloudHypervisorService struct {
	config *Config

	networkSvc ports.NetworkService
	fs         afero.Fs
}

func New(cfg *Config, networkSvc ports.NetworkService, fs afero.Fs) ports.MicroVMService {
	return &CloudHypervisorService{
		config:     cfg,
		networkSvc: networkSvc,
		fs:         fs,
	}
}

func (c *CloudHypervisorService) Start(ctx context.Context, vm *models.MicroVM, completionFn func(error)) (retErr error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "cloudhypervisor_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Debugf("creating microvm inside cloudhypervisor start")

	if vm.Spec.Kernel == "" || vm.Spec.RootfsPath == "" || vm.Spec.HostNetDev == "" || vm.Spec.GuestMAC == "" || vm.Spec.ImagePath == "" {
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

	defer func() {
		if retErr != nil {
			if err := c.networkSvc.IfaceDelete(ctx, ports.DeleteIfaceInput{
				DeviceName: status.HostDeviceName,
			}); err != nil {
				retErr = multierror.Append(retErr, err)
			}
		}
	}()

	proc, err := c.startMicroVM(vm, vmState, status, completionFn)

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

func (c *CloudHypervisorService) startMicroVM(vm *models.MicroVM, vmState *State, status *models.NetworkInterfaceStatus, completionFn func(error)) (*os.Process, error) {
	kernelCmdLine := DefaultKernelCmdLine()
	kernelCmdLine.Set("ip", fmt.Sprintf("%s::%s:%s::eth0::off", status.TapDetails.VmIp.To4(), status.TapDetails.TapIp.To4(), status.TapDetails.Mask.To4()))

	args := []string{
		"--log-file",
		vmState.LogPath(),
		"-v",
		"--serial", "tty",
		"--console", "off",
		"--cmdline", kernelCmdLine.String(),
		// 3 is the first unreserved CID
		"--vsock", fmt.Sprintf("cid=%d,socket=%s", 3, c.VSockPath(vm)),
		"--kernel", vm.Spec.Kernel,
		"--cpus", fmt.Sprintf("boot=%d", vm.Spec.VCPU),
		"--memory", fmt.Sprintf("size=%dM", vm.Spec.MemoryInMb),
		"--disk", fmt.Sprintf("path=%s,readonly=on", vm.Spec.RootfsPath), fmt.Sprintf("path=%s", vm.Spec.ImagePath),
		"--net", fmt.Sprintf("tap=%s,mac=%s,ip=%s,mask=%s",
			status.HostDeviceName,
			status.MACAddress,
			status.TapDetails.TapIp.To4(),
			status.TapDetails.Mask.To4()),
	}

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
	go func() { completionFn(cmd.Wait()) }()

	return cmd.Process, nil
}

func (c *CloudHypervisorService) ensureState(vmState *State) error {
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

func (c *CloudHypervisorService) Stop(ctx context.Context, vm *models.MicroVM) error {
	vmState := NewState(vm.ID, c.config.StateRoot, c.fs)

	pid, err := vmState.PID()
	if err != nil {
		return fmt.Errorf("failed to get PID: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process with pid %d: %w", pid, err)
	}

	retErr := proc.Kill()

	state, err := vmState.RuntimeState()
	if err != nil {
		retErr = multierror.Append(retErr, err)
	} else if err = c.networkSvc.IfaceDelete(context.Background(), ports.DeleteIfaceInput{
		DeviceName: state.HostIface,
	}); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	if err := vmState.Delete(); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	return retErr
}

func (c *CloudHypervisorService) Pid(ctx context.Context, vm *models.MicroVM) (int, error) {
	return NewState(vm.ID, c.config.StateRoot, c.fs).PID()
}

func (c *CloudHypervisorService) VSockPath(vm *models.MicroVM) string {
	return NewState(vm.ID, c.config.StateRoot, c.fs).VSockPath()
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
