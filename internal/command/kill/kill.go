package kill

import (
	"context"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	cmdflags "vistara-node/internal/command/flags"
	"vistara-node/internal/config"
	"vistara-node/pkg/api/services/microvm"
	"vistara-node/pkg/flags"
)

func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "kill",
		Short: "kill a VM",
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
	conn, err := grpc.NewClient(cfg.GRPCAPIEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close()

	request := microvm.DeleteMicroVMRequest{
		Id: os.Args[2],
	}

	vmServiceClient := microvm.NewVMServiceClient(conn)
	_, err = vmServiceClient.Delete(ctx, &request)

	if err != nil {
		return err
	}

	return nil
}
