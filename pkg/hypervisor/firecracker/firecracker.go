package firecracker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"

	"github.com/hashicorp/go-multierror"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/spf13/afero"
)

const (
	HypervisorName = "firecracker"
)

// Config represents the configuration options for the Firecracker infrastructure.
type Config struct {
	// FirecrackerBin is the firecracker binary to use.
	FirecrackerBin string
	// StateRoot is the folder to store any required firecracker state (i.e. socks, pid, log files).
	StateRoot string
}

type Service struct {
	config *Config
	fs     afero.Fs
}

// *services.MicroVMService
func New(cfg *Config, fs afero.Fs) ports.MicroVMService {
	return &Service{
		config: cfg,
		fs:     fs,
	}
}

func (f *Service) Start(_ context.Context, vm *models.MicroVM, completionFn func(error)) (retErr error) {
	if vm.Spec.Kernel == "" || vm.Spec.RootfsPath == "" || vm.Spec.HostNetDev == "" || vm.Spec.GuestMAC == "" || vm.Spec.ImagePath == "" {
		return errors.New("missing fields from model")
	}

	vmState := NewState(vm.ID, f.config.StateRoot, f.fs)

	if err := f.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	config, err := CreateConfig(WithMicroVM(vm, f.VSockPath(vm)), WithState(vmState))
	if err != nil {
		return fmt.Errorf("creating firecracker config: %w", err)
	}

	if err = vmState.SetConfig(config); err != nil {
		return fmt.Errorf("saving firecracker config %w", err)
	}
	meta := &Metadata{}

	if err = vmState.SetMetadata(meta); err != nil {
		return fmt.Errorf("saving firecracker metadata %w", err)
	}

	args := []string{"--boot-timer", "--no-api"}
	args = append(args, "--config-file", vmState.ConfigPath())
	args = append(args, "--metadata", vmState.MetadataPath())

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(f.config.FirecrackerBin).
		WithArgs(args).
		Build(context.Background())

	proc, err := f.startMicroVM(cmd, vmState, completionFn)

	if err != nil {
		return fmt.Errorf("starting firecracker process %w", err)
	}

	if err = vmState.SetPid(proc.Pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", proc.Pid, err)
	}

	return nil
}

func (f *Service) startMicroVM(cmd *exec.Cmd, vmState *State, completionFn func(error)) (*os.Process, error) {
	stdOutFile, err := f.fs.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
	}

	stdErrFile, err := f.fs.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening sterr file %s: %w", vmState.StderrPath(), err)
	}

	cmd.Stderr = stdErrFile
	cmd.Stdout = stdOutFile
	cmd.Stdin = &bytes.Buffer{}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting firecracker process: %w", err)
	}

	// Reap the process
	go func() { completionFn(cmd.Wait()) }()

	return cmd.Process, nil
}

func (f *Service) ensureState(vmState *State) error {
	exists, err := afero.DirExists(f.fs, vmState.Root())
	if err != nil {
		return fmt.Errorf("checking if state dir %s exists: %w", vmState.Root(), err)
	}

	if !exists {
		if err = f.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
			return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
		}
	}

	logFile, err := f.fs.OpenFile(vmState.LogPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening log file %s: %w", vmState.LogPath(), err)
	}

	defer logFile.Close()

	metricsFile, err := f.fs.OpenFile(vmState.MetricsPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening metrics file %s: %w", vmState.MetricsPath(), err)
	}

	defer metricsFile.Close()

	return nil
}

func (f *Service) Pid(_ context.Context, vm *models.MicroVM) (int, error) {
	vmState := NewState(vm.ID, f.config.StateRoot, f.fs)

	return vmState.PID()
}

func (f *Service) VSockPath(vm *models.MicroVM) string {
	return NewState(vm.ID, f.config.StateRoot, f.fs).VSockPath()
}

func (f *Service) Stop(_ context.Context, vm *models.MicroVM) error {
	vmState := NewState(vm.ID, f.config.StateRoot, f.fs)

	pid, err := vmState.PID()
	if err != nil {
		return fmt.Errorf("failed to get PID: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process with pid %d: %w", pid, err)
	}

	retErr := proc.Kill()

	if err = vmState.Delete(); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	return retErr
}
