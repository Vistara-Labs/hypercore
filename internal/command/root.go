package command

import (
	"fmt"
	"vistara-node/internal/command/kill"
	"vistara-node/internal/command/list"
	"vistara-node/internal/command/run"
	"vistara-node/internal/command/spawn"
	"vistara-node/internal/config"

	"vistara-node/internal/version"
	"vistara-node/pkg/flags"
	"vistara-node/pkg/log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRootCommand() (*cobra.Command, error) {
	cfg := &config.Config{}

	cmd := &cobra.Command{
		Use:   "vs",
		Short: "Hypercore - Vistara node",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			flags.BindCommandToViper(cmd)

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

	if err := addRootSubCommands(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding subcommands: %w", err)
	}

	cobra.OnInitialize(initCobra)

	return cmd, nil
}

func initCobra() {
	viper.SetEnvPrefix("VISTARAD")
	viper.AutomaticEnv()
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	// viper.AddConfigPath(defaults.ConfigurationDir)

	viper.AddConfigPath("$HOME/.config/vistarad/")

	_ = viper.ReadInConfig()
}

// Add run new command to the root command
func addRootSubCommands(cmd *cobra.Command, cfg *config.Config) error {
	runCmd, err := run.NewCommand(cfg)
	if err != nil {
		return fmt.Errorf("creating run cobra command: %w", err)
	}

	spawnCmd, err := spawn.NewCommand(cfg)
	if err != nil {
		return fmt.Errorf("creating spawn command: %w", err)
	}

	killCmd, err := kill.NewCommand(cfg)
	if err != nil {
		return fmt.Errorf("creating kill command: %w", err)
	}

	listCmd, err := list.NewCommand(cfg)
	if err != nil {
		return fmt.Errorf("creating list command: %w", err)
	}

	cmd.AddCommand(runCmd)
	cmd.AddCommand(spawnCmd)
	cmd.AddCommand(killCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(versionCommand())

	return nil
}

func versionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of vistara",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				long, short bool
				err         error
			)

			if long, err = cmd.Flags().GetBool("long"); err != nil {
				return err
			}

			if short, err = cmd.Flags().GetBool("short"); err != nil {
				return err
			}

			if short {
				fmt.Fprintln(cmd.OutOrStdout(), version.Version)

				return nil
			}

			if long {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\n  Version:    %s\n  CommitHash: %s\n  BuildDate:  %s\n",
					version.PackageName,
					version.Version,
					version.CommitHash,
					version.BuildDate,
				)

				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", version.PackageName, version.Version)

			return nil
		},
	}

	_ = cmd.Flags().Bool("long", false, "Print long version information")
	_ = cmd.Flags().Bool("short", false, "Print short version information")

	return cmd
}
