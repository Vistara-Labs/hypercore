package containerd

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/netns"
	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type CreateContainerOpts struct {
	ImageRef    string
	Snapshotter string
	Runtime     struct {
		Name    string
		Options interface{}
	}
	Limits *struct {
		CPUFraction float64
		MemoryBytes uint64
	}
	Labels     map[string]string
	CioCreator cio.Creator
}

type Repo struct {
	client *containerd.Client
	config *Config
}

func NewMicroVMRepository(cfg *Config) (*Repo, error) {
	client, err := containerd.New(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}

	return &Repo{
		client: client,
		config: cfg,
	}, nil
}

func (r *Repo) Attach(ctx context.Context, containerID string) error {
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

func (r *Repo) GetTasks(ctx context.Context) ([]*task.Process, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.ContainerNamespace)

	resp, err := r.client.TaskService().List(namespaceCtx, &tasks.ListTasksRequest{})
	if err != nil {
		return nil, err
	}

	return resp.GetTasks(), nil
}

func (r *Repo) GetContainer(ctx context.Context, id string) (containers.Container, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.ContainerNamespace)
	container, err := r.client.ContainerService().Get(namespaceCtx, id)
	if err != nil {
		return containers.Container{}, err
	}

	return container, nil
}

func (r *Repo) CreateContainer(ctx context.Context, opts CreateContainerOpts) (_ string, retErr error) {
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

	containerID := uuid.NewString()

	netNs, err := netns.NewNetNS("/run/netns")
	if err != nil {
		return "", fmt.Errorf("failed to create new net ns: %w", err)
	}

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithLinuxNamespace(specs.LinuxNamespace{Type: "network", Path: netNs.GetPath()}),
	}
	if opts.Limits != nil {
		specOpts = append(
			specOpts,
			oci.WithMemoryLimit(opts.Limits.MemoryBytes),
			// Quota is valid for every 100ms
			// https://docs.docker.com/engine/containers/resource_constraints/#configure-the-default-cfs-scheduler
			oci.WithCPUCFS(int64(opts.Limits.CPUFraction*100000), 100000),
		)
	}

	container, err := r.client.NewContainer(
		namespaceCtx,
		containerID,
		containerd.WithImage(image),
		containerd.WithSnapshotter(opts.Snapshotter),
		containerd.WithNewSnapshot(uuid.NewString(), image),
		containerd.WithRuntime(opts.Runtime.Name, opts.Runtime.Options),
		containerd.WithContainerLabels(opts.Labels),
		containerd.WithNewSpec(specOpts...),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create new container %s: %w", containerID, err)
	}

	defer func() {
		if retErr != nil {
			if err := container.Delete(namespaceCtx, containerd.WithSnapshotCleanup); err != nil {
				retErr = multierror.Append(retErr, err)
			}
		}
	}()

	ptpConfig := `
      {
        "type": "ptp",
        "ipMasq": true,
        "ipam": {
          "type": "host-local",
          "subnet": "192.168.127.0/24"
        },
        "dns": {
          "nameservers": ["1.1.1.1"]
        }
      }
    `
	firewallConfig := `{"type": "firewall"}`
	tapConfig := `{"type": "tc-redirect-tap"}`

	cniPlugins := []*libcni.NetworkConfig{
		{Network: &types.NetConf{Type: "ptp"}, Bytes: []byte(ptpConfig)},
		{Network: &types.NetConf{Type: "firewall"}, Bytes: []byte(firewallConfig)},
	}

	if opts.Runtime.Name == "hypercore.example" {
		cniPlugins = append(cniPlugins, &libcni.NetworkConfig{Network: &types.NetConf{Type: "tc-redirect-tap"}, Bytes: []byte(tapConfig)})
	}

	_, err = libcni.NewCNIConfig([]string{"/opt/cni/bin"}, nil).AddNetworkList(
		namespaceCtx, &libcni.NetworkConfigList{
			Name:       "hypercore-cni",
			CNIVersion: "0.4.0",
			Plugins:    cniPlugins,
		}, &libcni.RuntimeConf{
			ContainerID: containerID,
			NetNS:       netNs.GetPath(),
			IfName:      "eth0",
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to add CNI network list: %w", err)
	}

	task, err := container.NewTask(namespaceCtx, opts.CioCreator)
	if err != nil {
		return "", fmt.Errorf("failed to start task for container %s: %w", containerID, err)
	}

	defer func() {
		if retErr != nil {
			if _, err := task.Delete(namespaceCtx); err != nil {
				retErr = multierror.Append(retErr, err)
			}
		}
	}()

	err = task.Start(namespaceCtx)
	if err != nil {
		return "", fmt.Errorf("failed to start task for container %s: %w", containerID, err)
	}

	return containerID, nil
}

func (r *Repo) DeleteContainer(ctx context.Context, containerID string) (uint32, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.ContainerNamespace)

	container, err := r.client.LoadContainer(namespaceCtx, containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to load container %s: %w", containerID, err)
	}

	task, err := container.Task(namespaceCtx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get task: %w", err)
	}

	statusC, err := task.Wait(namespaceCtx)
	if err != nil {
		return 0, fmt.Errorf("failed to get exit status chan for container %s task: %w", containerID, err)
	}

	// If the task was not found, we can just stop the container
	if err := task.Kill(namespaceCtx, syscall.SIGTERM); err != nil {
		// TODO use github.com/containerd/errdefs
		if !strings.Contains(err.Error(), "not found") {
			return 0, fmt.Errorf("failed to kill task: %w", err)
		}
	}

	var code uint32

	select {
	case status := <-statusC:
		code, _, err = status.Result()
	case <-time.After(time.Second * 5):
		if err := task.Kill(namespaceCtx, syscall.SIGKILL); err != nil {
			return 0, fmt.Errorf("failed to kill task: %w", err)
		}

		code, _, err = (<-statusC).Result()
	}

	if err != nil {
		return 0, fmt.Errorf("failed to get exit status: %w", err)
	}

	log.WithContext(ctx).Infof("container %s exited with status %d", containerID, code)

	if _, err := task.Delete(namespaceCtx); err != nil {
		return 0, fmt.Errorf("failed to delete task: %w", err)
	}

	if err := container.Delete(namespaceCtx, containerd.WithSnapshotCleanup); err != nil {
		return 0, fmt.Errorf("failed to delete container %s: %w", containerID, err)
	}

	return code, nil
}
