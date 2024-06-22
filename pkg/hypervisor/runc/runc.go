package runc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"vistara-node/pkg/api/types"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"

	"github.com/containerd/containerd/cio"
	"github.com/spf13/afero"
)

// runc is not a hypervisor, but we implement the same interface
// as cloudhypervisor/firecracker so we follow the same format
const (
	HypervisorName = "runc"
)

// Config represents the configuration options for the Firecracker infrastructure.
type Config struct {
	// StateRoot is the folder to store any required state
	StateRoot string
}

type RuncService struct {
	config *Config

	containerd    ports.MicroVMRepository
	fs            afero.Fs
	idToContainer map[string]string
}

func New(cfg *Config, containerd ports.MicroVMRepository, fs afero.Fs) ports.MicroVMService {
	return &RuncService{
		config:        cfg,
		containerd:    containerd,
		fs:            fs,
		idToContainer: make(map[string]string),
	}
}

func (r *RuncService) Start(ctx context.Context, vm *models.MicroVM) error {
	vmState := NewState(vm.ID, r.config.StateRoot, r.fs)

	if vm.Spec.ImageRef == "" {
		return errors.New("image ref missing from model")
	}

	if err := r.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	stdOutFile, err := r.fs.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
	}

	stdErrFile, err := r.fs.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening stderr file %s: %w", vmState.StderrPath(), err)
	}

	cioCreator := cio.NewCreator(cio.WithStreams(&bytes.Buffer{}, stdOutFile, stdErrFile))

	containerId, err := r.containerd.CreateContainer(ctx, "", cioCreator)
	r.idToContainer[vm.ID.String()] = containerId

	return nil
}

func (r *RuncService) ensureState(vmState State) error {
	if err := r.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
		return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
	}

	return nil
}

func (r *RuncService) GetRuntimeData(ctx context.Context, vm *models.MicroVM) (*types.MicroVMRuntimeData, error) {
	return nil, nil
}

func (r *RuncService) Stop(ctx context.Context, vm *models.MicroVM) error {
	vmState := NewState(vm.ID, r.config.StateRoot, r.fs)

	containerId, ok := r.idToContainer[vm.ID.String()]
	if !ok {
		return fmt.Errorf("no container ID found for vm %s", vm.ID.String())
	}

	_, err := r.containerd.DeleteContainer(ctx, containerId)

	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerId, err)
	}

	if err = vmState.Delete(); err != nil {
		return fmt.Errorf("failed to delete vmState dir: %w", err)
	}

	return nil
}

func (r *RuncService) State(ctx context.Context, id string) (ports.MicroVMState, error) {
	return ports.MicroVMStateRunning, nil
}

func (r *RuncService) Metrics(ctx context.Context, id models.VMID) (ports.MachineMetrics, error) {
	machineMetrics := shared.MachineMetrics{}
	return machineMetrics, nil
}
