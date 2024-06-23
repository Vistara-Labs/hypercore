package docker

import (
	"context"
	"errors"
	"fmt"
	"vistara-node/pkg/api/types"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// docker is not a hypervisor, but we implement the same interface
// as cloudhypervisor/firecracker so we follow the same format
const (
	HypervisorName = "docker"
)

type DockerService struct {
	docker        *client.Client
	idToContainer map[string]string
}

func New() (ports.MicroVMService, error) {
	client, err := client.NewClientWithOpts()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	client.NegotiateAPIVersion(context.Background())

	return &DockerService{
		docker:        client,
		idToContainer: make(map[string]string),
	}, nil
}

func (d *DockerService) Start(ctx context.Context, vm *models.MicroVM) error {
	if vm.Spec.ImageRef == "" {
		return errors.New("image ref missing from model")
	}

	readCloser, err := d.docker.ImagePull(ctx, vm.Spec.ImageRef, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	readCloser.Close()

	containerResp, err := d.docker.ContainerCreate(
		ctx,
		&container.Config{
			Image: vm.Spec.ImageRef,
		},
		&container.HostConfig{
			AutoRemove: true,
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	if err = d.docker.ContainerStart(ctx, containerResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	d.idToContainer[vm.ID.String()] = containerResp.ID

	return nil
}

func (d *DockerService) GetRuntimeData(ctx context.Context, vm *models.MicroVM) (*types.MicroVMRuntimeData, error) {
	return nil, nil
}

func (d *DockerService) Stop(ctx context.Context, vm *models.MicroVM) error {
	containerId, ok := d.idToContainer[vm.ID.String()]
	if !ok {
		return fmt.Errorf("no container ID found for vm %s", vm.ID.String())
	}

	timeout := 15
	err := d.docker.ContainerStop(ctx, containerId, container.StopOptions{Timeout: &timeout})

	if err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerId, err)
	}

	return nil
}

func (d *DockerService) State(ctx context.Context, id string) (ports.MicroVMState, error) {
	return ports.MicroVMStateRunning, nil
}

func (d *DockerService) Metrics(ctx context.Context, id models.VMID) (ports.MachineMetrics, error) {
	machineMetrics := shared.MachineMetrics{}
	return machineMetrics, nil
}
