package spawn

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/typeurl/v2"
	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	cmdflags "vistara-node/internal/command/flags"
	"vistara-node/internal/config"
	"vistara-node/pkg/containerd"
	"vistara-node/pkg/flags"
	"vistara-node/pkg/models"
)

type HacConfig struct {
	Spacecore struct {
		name        string
		description string
	}
	Hardware struct {
		Cores     int32
		Memory    int32
		Kernel    string
		Drive     string
		Interface string
		Ref       string
	}
}

func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "spawn",
		Short: "Spawn a VM under Hypercore",
		PreRunE: func(c *cobra.Command, _ []string) error {
			flags.BindCommandToViper(c)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cfg)
		},
	}

	cmdflags.AddSpawnFlags(cmd, cfg)
	return cmd, nil
}

func run(ctx context.Context, cfg *config.Config) error {
	typeurl.Register(&models.MicroVMSpec{}, "models.MicroVMSpec")

	repo, err := containerd.NewMicroVMRepository(&containerd.Config{
		SnapshotterKernel:  cfg.CtrSnapshotterKernel,
		SnapshotterVolume:  "",
		SocketPath:         cfg.CtrSocketPath,
		Namespace:          cfg.CtrNamespace,
		ContainerNamespace: cfg.CtrNamespace + "-container",
	})
	if err != nil {
		return err
	}

	hacPath, err := filepath.Abs(cfg.HACFile)
	if err != nil {
		return err
	}

	hacContents, err := os.ReadFile(hacPath)
	if err != nil {
		return err
	}

	vmUUID := uuid.NewString()
	hacConfig := HacConfig{}

	toml.Unmarshal(hacContents, &hacConfig)

	fmt.Printf("Creating VM '%s' with config %+v\n", vmUUID, hacConfig)

	var id string

	switch cfg.DefaultVMProvider {
	case "runc":
		id, err = repo.CreateContainer(ctx, containerd.CreateContainerOpts{
			ImageRef:    hacConfig.Hardware.Ref,
			Snapshotter: "",
			Runtime: struct {
				Name    string
				Options interface{}
			}{
				Name: "io.containerd.runc.v2",
			},
			CioCreator: cio.NewCreator(),
		})
	case "firecracker":
		fallthrough
	case "cloudhypervisor":
		id, err = repo.CreateContainer(ctx, containerd.CreateContainerOpts{
			ImageRef:    hacConfig.Hardware.Ref,
			Snapshotter: "blockfile",
			Runtime: struct {
				Name    string
				Options interface{}
			}{
				Name: "hypercore.example",
				Options: &models.MicroVMSpec{
					Provider:   cfg.DefaultVMProvider,
					VCPU:       hacConfig.Hardware.Cores,
					MemoryInMb: hacConfig.Hardware.Memory,
					HostNetDev: hacConfig.Hardware.Interface,
					Kernel:     hacConfig.Hardware.Kernel,
					RootfsPath: hacConfig.Hardware.Drive,
				},
			},
			CioCreator: cio.NewCreator(),
		})
	case "docker":
		panic("TODO")
	}

	if err != nil {
		return err
	}

	fmt.Printf("ID: %s\n", id)
	return nil
}
