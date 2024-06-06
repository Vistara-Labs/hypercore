package firecracker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/network"
	"vistara-node/pkg/ports"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
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
	// RunDetached indicates that the firecracker processes should be run detached (a.k.a daemon) from the parent process.
	RunDetached bool
	// DeleteVMTimeout is the timeout to wait for the microvm to be deleted.
	DeleteVMTimeout time.Duration
}

type FirecrackerService struct {
	config *Config
	// create a network service that interacts with the network stack on the host machine
	networkSvc ports.NetworkService
	fs         afero.Fs
}

// Reconcile implements ports.MicroVMService.
func (*FirecrackerService) Reconcile(ctx context.Context, vmid models.VMID) error {
	panic("unimplemented")
}

// *services.MicroVMService
func New(cfg *Config, networkSvc ports.NetworkService, fs afero.Fs) ports.MicroVMService {
	return &FirecrackerService{
		config:     cfg,
		networkSvc: networkSvc,
		fs:         fs,
	}
}

func (f *FirecrackerService) Start(ctx context.Context, vm *models.MicroVM) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Debugf("creating microvm inside firecracker start")

	vmState := NewState(vm.ID, f.config.StateRoot, f.fs)

	if err := f.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	// We will have only one interface, i.e. the TAP device
	iface := vm.Spec.NetworkInterfaces[0]
	status := &models.NetworkInterfaceStatus{}

	vm.Status.NetworkInterfaces = make(map[string]*models.NetworkInterfaceStatus)
	vm.Status.NetworkInterfaces[iface.GuestDeviceName] = status

	nface := network.NewNetworkInterface(&vm.ID, &iface, status, f.networkSvc)
	if err := nface.Create(ctx); err != nil {
		return fmt.Errorf("creating network interface %w", err)
	}

	config, err := CreateConfig(WithMicroVM(vm, status), WithState(vmState))
	if err != nil {
		return fmt.Errorf("creating firecracker config: %w", err)
	}
	if err = vmState.SetConfig(config); err != nil {
		return fmt.Errorf("saving firecracker config %W", err)
	}
	meta := &Metadata{
		Latest: vm.Spec.Metadata,
	}

	if err = vmState.SetMetadata(meta); err != nil {
		return fmt.Errorf("saving firecracker metadata %w", err)
	}

	args := []string{"--id", vm.ID.UID(), "--boot-timer", "--no-api"}
	args = append(args, "--config-file", vmState.ConfigPath())
	args = append(args, "--metadata", vmState.MetadataPath())

	// cmd := firecracker.VMCommandBuilder{}.
	// 	WithBin(f.config.FirecrackerBin).
	// 	WithArgs(args).
	// 	Build(context.TODO())
	// cmd := exec.Command(f.config.FirecrackerBin, args...)

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(f.config.FirecrackerBin).
		WithArgs(args).
		Build(context.TODO()) //nolint: contextcheck // Intentional.

	proc, err := f.startMicroVM(cmd, vmState, f.config.RunDetached)

	if err != nil {
		return fmt.Errorf("starting firecracker process %w", err)
	}

	if err = vmState.SetPid(proc.Pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", proc.Pid, err)
	}
	return nil
}

func (f *FirecrackerService) startMicroVM(cmd *exec.Cmd, vmState State, detached bool) (*os.Process, error) {
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

	var startErr error

	if detached && false {
		startErr = nil //process.DetachedStart(cmd)
	} else {
		startErr = cmd.Start()
	}

	if startErr != nil {
		return nil, fmt.Errorf("starting firecracker process: %w", startErr)
	}

	return cmd.Process, nil
}

func (f *FirecrackerService) ensureState(vmState State) error {
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

	logFile.Close()

	metricsFile, err := f.fs.OpenFile(vmState.MetricsPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening metrics file %s: %w", vmState.MetricsPath(), err)
	}

	metricsFile.Close()

	return nil
}

func (f *FirecrackerService) Create(ctx context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
	// Implement Firecracker stop logic
	return nil, nil
}

func (f *FirecrackerService) Delete(ctx context.Context, id string) error {
	// Implement Firecracker status check logic
	return nil
}

func (f *FirecrackerService) State(ctx context.Context, id string) (ports.MicroVMState, error) {
	// Implement Firecracker status check logic
	return ports.MicroVMStateRunning, nil
}

func (f *FirecrackerService) Metrics(ctx context.Context, id models.VMID) (ports.MachineMetrics, error) {
	// Implement Firecracker status check logic
	machineMetrics := shared.MachineMetrics{}
	return machineMetrics, nil
}
