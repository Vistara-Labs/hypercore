package spawn

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"path/filepath"
	cmdflags "vistara-node/internal/command/flags"
	"vistara-node/internal/config"
	"vistara-node/pkg/api/services/microvm"
	"vistara-node/pkg/api/types"
	"vistara-node/pkg/flags"
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
	conn, err := grpc.NewClient(cfg.GRPCAPIEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close()

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

	// TODO figure out how to get the appropriate MAC for a VM image
	guestMac := "06:00:AC:10:00:02"

	request := microvm.CreateMicroVMRequest{
		Microvm: &types.MicroVMSpec{
			Id:         vmUUID,
			Vcpu:       hacConfig.Hardware.Cores,
			MemoryInMb: hacConfig.Hardware.Memory,
			KernelPath: hacConfig.Hardware.Kernel,
			RootfsPath: hacConfig.Hardware.Drive,
			GuestMac:   guestMac,
			HostNetDev: hacConfig.Hardware.Interface,
			Provider:   cfg.DefaultVMProvider,
		},
	}

	vmServiceClient := microvm.NewVMServiceClient(conn)
	response, err := vmServiceClient.Create(ctx, &request)

	if err != nil {
		return err
	}

	fmt.Printf("Response: %v\n", response)
	return nil
}
