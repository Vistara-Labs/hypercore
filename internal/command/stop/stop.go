package stop

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	cmdflags "vistara-node/internal/command/flags"
	"vistara-node/internal/config"
	"vistara-node/pkg/containerd"
	"vistara-node/pkg/flags"
)

func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop a VM",
		Args:  cobra.ExactArgs(1),
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

	code, err := repo.DeleteContainer(ctx, os.Args[2])
	if err != nil {
		return err
	}

	os.Exit(int(code))
	return nil
}
