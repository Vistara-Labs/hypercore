// +build darwin

package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
)

// Stub implementations for Mac - cluster commands don't need full containerd functionality

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
	Env        []string
}

type Repo struct {
	client *containerd.Client
	config *Config
}

func NewMicroVMRepository(cfg *Config) (*Repo, error) {
	return nil, fmt.Errorf("containerd operations not supported on Mac - use cluster commands only")
}

func (r *Repo) CreateContainer(ctx context.Context, opts CreateContainerOpts) (string, error) {
	return "", fmt.Errorf("containerd operations not supported on Mac")
}

func (r *Repo) GetContainer(ctx context.Context, id string) (containerd.Container, error) {
	return nil, fmt.Errorf("containerd operations not supported on Mac")
}

func (r *Repo) DeleteContainer(ctx context.Context, id string) (uint32, error) {
	return 0, fmt.Errorf("containerd operations not supported on Mac")
}

func (r *Repo) GetTasks(ctx context.Context) ([]*task.Process, error) {
	return nil, fmt.Errorf("containerd operations not supported on Mac")
}

func (r *Repo) Attach(ctx context.Context, id string) error {
	return fmt.Errorf("containerd operations not supported on Mac")
}

func (r *Repo) GetContainerPrimaryIP(ctx context.Context, id string) (string, error) {
	return "", fmt.Errorf("containerd operations not supported on Mac")
}

func (r *Repo) GetContext(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, r.config.ContainerNamespace)
}

func (r *Repo) GarbageCollectCNI(ctx context.Context) (int, error) {
	return 0, nil // No-op on Mac
}
