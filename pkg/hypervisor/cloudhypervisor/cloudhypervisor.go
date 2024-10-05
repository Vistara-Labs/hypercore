package cloudhypervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/models"
	"vistara-node/pkg/network"
	"vistara-node/pkg/ports"

	"github.com/hashicorp/go-multierror"

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

type Service struct {
	config *Config

	fs afero.Fs
}

func New(cfg *Config, fs afero.Fs) ports.MicroVMService {
	return &Service{
		config: cfg,
		fs:     fs,
	}
}

func (c *Service) Start(_ context.Context, vm *models.MicroVM, completionFn func(error)) (retErr error) {
	if vm.Spec.Kernel == "" || vm.Spec.RootfsPath == "" || vm.Spec.HostNetDev == "" || vm.Spec.GuestMAC == "" || vm.Spec.ImagePath == "" {
		return errors.New("missing fields from model")
	}

	vmState := NewState(vm.ID, c.config.StateRoot, c.fs)

	if err := c.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	proc, err := c.startMicroVM(vm, vmState, completionFn)

	if err != nil {
		return fmt.Errorf("starting cloudhypervisor process %w", err)
	}

	if err = vmState.SetPid(proc.Pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", proc.Pid, err)
	}

	if err = vmState.SetRuntimeState(RuntimeState{HostIface: "tap0"}); err != nil {
		return fmt.Errorf("saving runtime state: %w", err)
	}

	return nil
}

func (c *Service) startMicroVM(vm *models.MicroVM, vmState *State, completionFn func(error)) (*os.Process, error) {
	kernelCmdLine := DefaultKernelCmdLine()
	mac, ip, err := network.GetLinkMacIP("eth0")
	if err != nil {
		return nil, fmt.Errorf("failed to get link IP: %w", err)
	}

	ifaceIP := ip.String()
	// 192.168.127.X -> 192.168.127.1
	ip[3] = 1
	routeIP := ip.String()

	kernelCmdLine.Set("ip", fmt.Sprintf("%s::%s:%s::eth0::%s", ifaceIP, routeIP, network.MaskToString(ip.DefaultMask()), "1.1.1.1"))

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
			"tap0",
			mac.String(),
			ifaceIP,
			network.MaskToString(ip.DefaultMask())),
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

func (c *Service) ensureState(vmState *State) error {
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

func (c *Service) Stop(_ context.Context, vm *models.MicroVM) error {
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

	if err := vmState.Delete(); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	return retErr
}

func (c *Service) Pid(_ context.Context, vm *models.MicroVM) (int, error) {
	return NewState(vm.ID, c.config.StateRoot, c.fs).PID()
}

func (c *Service) VSockPath(vm *models.MicroVM) string {
	return NewState(vm.ID, c.config.StateRoot, c.fs).VSockPath()
}
