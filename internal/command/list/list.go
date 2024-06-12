package list

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	cmdflags "vistara-node/internal/command/flags"
	"vistara-node/internal/config"
	"vistara-node/pkg/api/services/microvm"
	"vistara-node/pkg/flags"
	"vistara-node/pkg/network"
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
	conn, err := grpc.NewClient(cfg.GRPCAPIEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close()

	vmServiceClient := microvm.NewVMServiceClient(conn)
	response, err := vmServiceClient.List(ctx, new(emptypb.Empty))

	if err != nil {
		return err
	}

	for _, vm := range response.Microvm {
		linkIp, err := network.GetLinkIp(vm.RuntimeData.NetworkInterface)
		if err != nil {
			return err
		}

		ip4 := linkIp.To4()
		ip4[3] = ip4[3] - 1

		fmt.Printf("VM %s, SSH Address %s\n", vm.Microvm.Spec.Id, ip4.String())
	}

	return nil
}
