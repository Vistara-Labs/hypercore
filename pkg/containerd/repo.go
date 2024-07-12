package containerd

import (
	"context"
	"fmt"
	"syscall"
	"vistara-node/pkg/log"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/google/uuid"
)

type RunningContainer struct {
	ExitStatusChan <-chan containerd.ExitStatus
	Task           containerd.Task
}

type CreateContainerOpts struct {
	ImageRef    string
	Snapshotter string
	Runtime     struct {
		Name    string
		Options interface{}
	}
	Labels     map[string]string
	CioCreator cio.Creator
}

type containerdRepo struct {
	client       *containerd.Client
	config       *Config
	containerMap map[string]RunningContainer
}

func NewMicroVMRepository(cfg *Config) (*containerdRepo, error) {
	client, err := containerd.New(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}

	return &containerdRepo{
		client:       client,
		config:       cfg,
		containerMap: make(map[string]RunningContainer),
	}, nil
}

func (r *containerdRepo) Attach(ctx context.Context, containerID string) error {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.ContainerNamespace)

	container, err := r.client.LoadContainer(namespaceCtx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}

	task, err := container.Task(namespaceCtx, cio.NewAttach(cio.WithStdio))
	if err != nil {
		return fmt.Errorf("failed to get task for container %s: %w", containerID, err)
	}

	statusC, err := task.Wait(namespaceCtx)
	if err != nil {
		return fmt.Errorf("failed to get status chan for task %s: %w", task.ID(), err)
	}

	// TODO tty, forward signals
	<-statusC

	return nil
}

func (r *containerdRepo) GetTasks(ctx context.Context) ([]*task.Process, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.ContainerNamespace)

	resp, err := r.client.TaskService().List(namespaceCtx, &tasks.ListTasksRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Tasks, nil
}

func (r *containerdRepo) CreateContainer(ctx context.Context, opts CreateContainerOpts) (_ string, retErr error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.ContainerNamespace)

	image, err := r.client.Pull(namespaceCtx, opts.ImageRef)
	if err != nil {
		return "", fmt.Errorf("failed to pull image %s: %w", opts.ImageRef, err)
	}

	unpacked, err := image.IsUnpacked(namespaceCtx, opts.Snapshotter)
	if err != nil {
		return "", fmt.Errorf("failed to check image unpack status: %w", err)
	}

	if !unpacked {
		if err := image.Unpack(namespaceCtx, opts.Snapshotter); err != nil {
			return "", fmt.Errorf("failed to unpack image with snapshotter %s: %w", opts.Snapshotter, err)
		}
	}

	// We don't want the context stored internally to get cancelled
	// when this request completes
	namespaceCtx = namespaces.WithNamespace(context.Background(), r.config.ContainerNamespace)

	containerId := uuid.NewString()

	container, err := r.client.NewContainer(
		namespaceCtx,
		containerId,
		containerd.WithImage(image),
		containerd.WithSnapshotter(opts.Snapshotter),
		containerd.WithNewSnapshot(uuid.NewString(), image),
		containerd.WithRuntime(opts.Runtime.Name, opts.Runtime.Options),
		containerd.WithContainerLabels(opts.Labels),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create new container %s: %w", containerId, err)
	}

	defer func() {
		if retErr != nil {
			container.Delete(namespaceCtx, containerd.WithSnapshotCleanup)
		}
	}()

	task, err := container.NewTask(namespaceCtx, opts.CioCreator)
	if err != nil {
		return "", fmt.Errorf("failed to start task for container %s: %w", containerId, err)
	}

	defer func() {
		if retErr != nil {
			task.Delete(namespaceCtx)
		}
	}()

	exitStatusChan, err := task.Wait(namespaceCtx)
	if err != nil {
		return "", fmt.Errorf("failed to get exit status chan for container %s task: %w", containerId, err)
	}

	err = task.Start(namespaceCtx)
	if err != nil {
		return "", fmt.Errorf("failed to start task for container %s: %w", containerId, err)
	}

	r.containerMap[containerId] = RunningContainer{
		ExitStatusChan: exitStatusChan,
		Task:           task,
	}

	return containerId, nil
}

func (r *containerdRepo) DeleteContainer(ctx context.Context, containerId string) (uint32, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.ContainerNamespace)

	container, exists := r.containerMap[containerId]
	if !exists {
		return 0, fmt.Errorf("container %s not found", containerId)
	}

	err := container.Task.Kill(namespaceCtx, syscall.SIGTERM)
	if err != nil {
		return 0, fmt.Errorf("failed to kill task: %w", err)
	}

	delete(r.containerMap, containerId)

	status := <-container.ExitStatusChan

	code, _, err := status.Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get exit status: %w", err)
	}

	log.GetLogger(ctx).Infof("container %s exited with status %d", containerId, code)

	return code, nil
}
