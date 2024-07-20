package hypercore

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"vistara-node/pkg/log"
)

func Run() {
	cfg := &Config{}
	cmd := &cobra.Command{
		Use:   "vs",
		Short: "Hypercore - Vistara node",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			BindCommandToViper(cmd)

			if err := log.Configure(&cfg.Logging); err != nil {
				return fmt.Errorf("configuring logging: %w", err)
			}

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}

	log.AddFlagsToCommand(cmd, &cfg.Logging)

	cmd.AddCommand(AttachCommand(cfg))
	cmd.AddCommand(ListCommand(cfg))
	cmd.AddCommand(SpawnCommand(cfg))
	cmd.AddCommand(StopCommand(cfg))

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
