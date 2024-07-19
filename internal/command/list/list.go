package list

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	cmdflags "vistara-node/internal/command/flags"
	"vistara-node/internal/config"
	"vistara-node/pkg/containerd"
	"vistara-node/pkg/flags"
)

func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List running VMs",
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

	tasks, err := repo.GetTasks(ctx)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		fmt.Printf("Task %s, Container %s\n", task.ID, task.ContainerID)
	}

	return nil
}
